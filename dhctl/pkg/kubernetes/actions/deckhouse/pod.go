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

package deckhouse

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func GetPod(kubeCl *client.KubernetesClient) (*v1.Pod, error) {
	pods, err := kubeCl.CoreV1().Pods("d8-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=deckhouse"})
	if err != nil {
		log.DebugF("Cannot get deckhouse pod. Got error: %v", err)
		return nil, ErrListPods
	}

	if len(pods.Items) == 0 {
		log.DebugF("Cannot get deckhouse pod. Count of returned pods is zero")
		return nil, ErrListPods
	}

	if len(pods.Items) > 1 {
		return nil, ErrMultiplePodsFound
	}

	pod := pods.Items[0]

	return &pod, nil
}

func GetRunningPod(kubeCl *client.KubernetesClient) (*v1.Pod, error) {
	pod, err := GetPod(kubeCl)
	if err != nil {
		return nil, err
	}

	phase := pod.Status.Phase
	message := fmt.Sprintf("Deckhouse pod found: %s (%s)", pod.Name, pod.Status.Phase)

	if phase != v1.PodRunning {
		return nil, fmt.Errorf(message)
	}

	return pod, nil
}

func DeletePod(kubeCl *client.KubernetesClient) error {
	pod, err := GetPod(kubeCl)
	if err != nil {
		return err
	}

	return kubeCl.CoreV1().Pods("d8-system").Delete(context.TODO(), pod.GetName(), metav1.DeleteOptions{})
}
