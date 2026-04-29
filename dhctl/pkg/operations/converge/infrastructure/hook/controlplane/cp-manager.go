// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var ErrControlPlaneIsNotReady = errors.New("control plane is not ready")

var requiredControlPlaneNodeConditions = []string{
	"EtcdReady",
	"APIServerReady",
	"ControllerManagerReady",
	"SchedulerReady",
	"CertificatesHealthy",
}

type ManagerReadinessChecker struct {
	getter kubernetes.KubeClientProvider
}

func NewManagerReadinessChecker(getter kubernetes.KubeClientProvider) *ManagerReadinessChecker {
	return &ManagerReadinessChecker{
		getter: getter,
	}
}

func (c *ManagerReadinessChecker) IsReadyAll(ctx context.Context) error {
	return retry.NewLoop("Control-plane readiness", 50, 10*time.Second).RunContext(ctx, func() error {
		msg, err := checkControlPlaneNodesReady(ctx, c.getter.KubeClient())

		// all ControlPlaneNodes are ready
		if err == nil {
			log.InfoLn(msg)
			return nil
		}

		// some ControlPlaneNodes are not ready
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}

		// some other error occurred
		log.DebugF("Error while checking control-plane nodes readiness: %v\n", err)
		return ErrControlPlaneIsNotReady
	})
}

func (c *ManagerReadinessChecker) IsReady(ctx context.Context, nodeName string) (bool, error) {
	conditions, err := getControlPlaneNodeConditions(ctx, c.getter.KubeClient(), nodeName)
	if err != nil {
		return false, err
	}

	return isControlPlaneNodeReady(conditions), nil
}

func (c *ManagerReadinessChecker) Name() string {
	return "Control plane readiness"
}

// checkControlPlaneNodesReady verifies that every master node has a ready ControlPlaneNode.
// Returns a short readiness summary and an error when at least one required condition is not True.
func checkControlPlaneNodesReady(ctx context.Context, kubeClient client.KubeClient) (string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node.deckhouse.io/group=master",
	})
	if err != nil {
		return "", fmt.Errorf("get nodes count: %w", err)
	}

	readyNodes := 0
	var msg strings.Builder

	for _, node := range nodes.Items {
		conditions, err := getControlPlaneNodeConditions(ctx, kubeClient, node.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				log.DebugF("Error while getting control-plane node %s readiness: %v\n", node.Name, err)
			}
			appendControlPlaneNodeReadinessMessage(&msg, node.Name, nil, err)
			continue
		}

		if isControlPlaneNodeReady(conditions) {
			readyNodes++
		}

		appendControlPlaneNodeReadinessMessage(&msg, node.Name, conditions, nil)
	}

	header := fmt.Sprintf("ControlPlaneNodes Ready %v of %v", readyNodes, len(nodes.Items))
	if msg.Len() > 0 {
		header = fmt.Sprintf("%s\n%s", header, msg.String())
	}

	if readyNodes >= len(nodes.Items) {
		return header, nil
	}

	return header, ErrControlPlaneIsNotReady
}

// isControlPlaneNodeReady checks if all required ControlPlaneNode conditions are True.
func isControlPlaneNodeReady(conditions []metav1.Condition) bool {
	conditionsByType := controlPlaneNodeConditionsByType(conditions)
	for _, conditionType := range requiredControlPlaneNodeConditions {
		condition, ok := conditionsByType[conditionType]
		if !ok || condition.Status != metav1.ConditionTrue {
			return false
		}
	}

	return true
}

// getControlPlaneNodeConditions retrieves a ControlPlaneNode by node name and returns its status conditions.
func getControlPlaneNodeConditions(ctx context.Context, kubeClient client.KubeClient, nodeName string) ([]metav1.Condition, error) {
	cpn, err := kubeClient.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "control-plane.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "controlplanenodes",
	}).Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get ControlPlaneNode %s: %w", nodeName, err)
	}

	return controlPlaneNodeConditions(cpn)
}

// controlPlaneNodeConditions converts unstructured ControlPlaneNode status.conditions to metav1.Condition.
func controlPlaneNodeConditions(cpn *unstructured.Unstructured) ([]metav1.Condition, error) {
	type controlPlaneNodeStatus struct {
		Conditions []metav1.Condition `json:"conditions"`
	}

	type controlPlaneNode struct {
		Status controlPlaneNodeStatus `json:"status"`
	}

	var obj controlPlaneNode
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(cpn.Object, &obj); err != nil {
		return nil, fmt.Errorf("convert ControlPlaneNode status: %w", err)
	}

	return obj.Status.Conditions, nil
}

// appendControlPlaneNodeReadinessMessage appends one diagnostic line for a ControlPlaneNode.
func appendControlPlaneNodeReadinessMessage(msg *strings.Builder, nodeName string, conditions []metav1.Condition, err error) {
	if msg.Len() > 0 {
		msg.WriteString("\n")
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			fmt.Fprintf(msg, "* %s: ControlPlaneNode not found", nodeName)
			return
		}

		fmt.Fprintf(msg, "* %s: %v", nodeName, err)
		return
	}

	conditionsByType := controlPlaneNodeConditionsByType(conditions)
	readyConditionTypes := make([]string, 0, len(requiredControlPlaneNodeConditions))
	for _, conditionType := range requiredControlPlaneNodeConditions {
		condition, ok := conditionsByType[conditionType]
		if ok && condition.Status == metav1.ConditionTrue {
			readyConditionTypes = append(readyConditionTypes, condition.Type)
		}
	}

	fmt.Fprintf(msg, "* %s: | %s", nodeName, strings.Join(readyConditionTypes, ", "))
}

func controlPlaneNodeConditionsByType(conditions []metav1.Condition) map[string]metav1.Condition {
	result := make(map[string]metav1.Condition, len(conditions))
	for _, condition := range conditions {
		result[condition.Type] = condition
	}

	return result
}
