/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package node

import (
	"context"
	"encoding/json"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
)

var logger = log.Default().Named("node")

type NodeManager struct {
	kubeClient kubernetes.Interface
}

func NewNodeManager(kubeClient kubernetes.Interface) *NodeManager {
	return &NodeManager{
		kubeClient: kubeClient,
	}
}

func (m *NodeManager) MarkAsControlPlane(nodeName string) error {
	logger.Info("Marking node as control plane with label and taint", slog.String("node", nodeName))
	newLabels := make(map[string]string)
	newLabels[constants.ControlPlaneLabelKey] = ""

	newTaints := make([]corev1.Taint, 0, 1)
	controlPlaneTaint := corev1.Taint{
		Key:    constants.ControlPlaneTaintKey,
		Value:  "",
		Effect: corev1.TaintEffectNoSchedule,
	}
	newTaints = append(newTaints, controlPlaneTaint)

	if err := m.setLabels(nodeName, newLabels); err != nil {
		log.Error("failed to set control-plane labels", slog.String("node", nodeName), slog.Any("labels", newLabels), slog.Any("error", err))
		return err
	}

	if err := m.setTaints(nodeName, newTaints); err != nil {
		log.Error("failed to set control-plane taints", slog.String("node", nodeName), slog.Any("taints", newTaints), slog.Any("error", err))
		return err
	}

	return nil
}

func (m *NodeManager) setLabels(nodeName string, labels map[string]string) error {
	ctx := context.Background()

	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		log.Error("failed to marshal label patch data", slog.Any("error", err))
	}

	_, err = m.kubeClient.CoreV1().Nodes().Patch(
		ctx,
		nodeName,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	return err
}

func (m *NodeManager) setTaints(nodeName string, taints []corev1.Taint) error {
	ctx := context.Background()

	node, err := m.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		log.Error("failed to get node", slog.String("node", nodeName), slog.Any("error", err))
		return err
	}

	// Compare new and existing taints by key via making a map
	existingTaints := make(map[string]corev1.Taint)
	for _, taint := range node.Spec.Taints {
		existingTaints[taint.Key] = taint
	}

	for _, newTaint := range taints {
		existingTaints[newTaint.Key] = newTaint
	}

	// Convert back to slice
	newTaints := make([]corev1.Taint, 0, len(existingTaints))
	for _, taint := range existingTaints {
		newTaints = append(newTaints, taint)
	}

	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": newTaints,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return err
	}

	_, err = m.kubeClient.CoreV1().Nodes().Patch(
		ctx,
		nodeName,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	return err
}
