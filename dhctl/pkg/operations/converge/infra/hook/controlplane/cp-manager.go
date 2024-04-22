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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var ErrControlPlaneIsNotReady = errors.New("Control plane is not ready")

type ManagerReadinessChecker struct {
	kubeCl *client.KubernetesClient
}

func NewManagerReadinessChecker(kubeCl *client.KubernetesClient) *ManagerReadinessChecker {
	return &ManagerReadinessChecker{
		kubeCl: kubeCl,
	}
}

func (c *ManagerReadinessChecker) IsReadyAll() error {
	return retry.NewLoop("Control-plane manager pods readiness", 25, 10*time.Second).Run(func() error {
		nodes, err := c.kubeCl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "node.deckhouse.io/group=master",
		})
		if err != nil {
			log.DebugF("Error while getting nodes count: %v\n", err)
			return ErrControlPlaneIsNotReady
		}

		cpmPodsList, err := c.kubeCl.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=d8-control-plane-manager",
		})
		if err != nil {
			log.DebugF("Error while getting control-plane manager pods: %v\n", err)
			return ErrControlPlaneIsNotReady
		}

		readyPods := make(map[string]struct{})
		message := fmt.Sprintf("Pods Ready %v of %v\n", len(cpmPodsList.Items), len(nodes.Items))
		for _, pod := range cpmPodsList.Items {
			p := pod
			ready, err := isPodReady(&p)
			condition := "NotReady"
			if err != nil {
				log.DebugF("Error while getting control-plane manager pod readiness: %v\n", err)
			}
			if ready {
				condition = "Ready"
				readyPods[p.Name] = struct{}{}
			}

			message += fmt.Sprintf("* %s (%s) | %s\n", p.Name, p.Spec.NodeName, condition)
		}

		if len(readyPods) >= len(nodes.Items) {
			log.InfoLn(message)
			return nil
		}

		return fmt.Errorf(strings.TrimSuffix(message, "\n"))
	})
}

func (c *ManagerReadinessChecker) IsReady(nodeName string) (bool, error) {
	cpmPodsList, err := c.kubeCl.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=d8-control-plane-manager",
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})

	if err != nil {
		return false, err
	}

	if len(cpmPodsList.Items) == 0 {
		return false, fmt.Errorf("Not found control plane manage pod")
	}

	if len(cpmPodsList.Items) > 1 {
		return false, fmt.Errorf("Found multiple control plane manager pods for one node")
	}

	return isPodReady(&cpmPodsList.Items[0])
}

func (c *ManagerReadinessChecker) Name() string {
	return "Control plane manager readiness"
}

func isPodReady(p *corev1.Pod) (bool, error) {
	podName := p.GetName()
	phase := p.Status.Phase

	if p.Status.Phase != corev1.PodRunning {
		return false, fmt.Errorf("Control plane manager pod %s is not running (%s)", podName, phase)
	}

	for _, status := range p.Status.ContainerStatuses {
		if status.Name != "control-plane-manager" {
			continue
		}

		return status.Ready, nil
	}

	return false, fmt.Errorf("Not found control-plane-manager container in pod %s", podName)
}
