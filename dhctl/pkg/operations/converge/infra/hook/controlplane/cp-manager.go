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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type ManagerReadinessChecker struct {
	kubeCl *client.KubernetesClient
}

func NewManagerReadinessChecker(kubeCl *client.KubernetesClient) *ManagerReadinessChecker {
	return &ManagerReadinessChecker{
		kubeCl: kubeCl,
	}
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

	cpmPod := cpmPodsList.Items[0]
	podName := cpmPod.GetName()
	phase := cpmPod.Status.Phase

	if cpmPod.Status.Phase != corev1.PodRunning {
		return false, fmt.Errorf("Control plane manager pod %s is not running (%s)", podName, phase)
	}

	for _, status := range cpmPod.Status.ContainerStatuses {
		if status.Name != "control-plane-manager" {
			continue
		}

		return status.Ready, nil
	}

	return false, fmt.Errorf("Not found control-plane-manager container in pod %s", podName)
}

func (c *ManagerReadinessChecker) Name() string {
	return "Control plane manager readiness"
}
