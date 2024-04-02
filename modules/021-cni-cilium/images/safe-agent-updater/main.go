/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ciliumNS             = "d8-cni-cilium"
	generationAnnotation = "safe-agent-updater-daemonset-generation"
	scanInterval         = 3 * time.Second
	scanIterations       = 20
)

func main() {
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[SafeAgentUpdater] Failed to init kubeClient. Error: %v", err)
	}
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		log.Fatalf("[SafeAgentUpdater] Failed to get env NODE_NAME.")
	}
	currentAgentPodName, isCurrentAgentPodGenerationDesired, err := checkAgentPodGeneration(kubeClient, nodeName)
	if err != nil {
		log.Fatal(err)
	}
	if !isCurrentAgentPodGenerationDesired {
		err = deletePod(kubeClient, currentAgentPodName)
		if err != nil {
			log.Fatal(err)
		}
		err := waitUntilNewPodCreatedAndBecomeReady(kubeClient, nodeName, scanIterations)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Infof("[SafeAgentUpdater] Finished and exit")
}

func checkAgentPodGeneration(kubeClient kubernetes.Interface, nodeName string) (currentAgentPodName string, isCurrentAgentPodGenerationDesired bool, err error) {
	ciliumAgentDS, err := kubeClient.AppsV1().DaemonSets(ciliumNS).Get(
		context.TODO(),
		"agent",
		metav1.GetOptions{},
	)
	if err != nil {
		return "", false, fmt.Errorf(
			"[SafeAgentUpdater] Failed to get DaemonSets %s/agent. Error: %v",
			ciliumNS,
			err,
		)
	}

	desiredAgentGeneration := ciliumAgentDS.Spec.Template.Annotations[generationAnnotation]
	if len(desiredAgentGeneration) == 0 {
		return "", false, fmt.Errorf(
			"[SafeAgentUpdater] DaemonSets %s/agent doesn't have annotation %s",
			ciliumNS,
			generationAnnotation,
		)
	}
	log.Infof(
		"[SafeAgentUpdater] Desired generation of agent is %s",
		desiredAgentGeneration,
	)

	ciliumAgentPodsOnSameNode, err := kubeClient.CoreV1().Pods(ciliumNS).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "app=agent",
			FieldSelector: "spec.nodeName=" + nodeName,
		},
	)
	if err != nil {
		return "", false, fmt.Errorf(
			"[SafeAgentUpdater] Failed to list pods on same node. Error: %v",
			err,
		)
	}

	log.Infof(
		"[SafeAgentUpdater] Count of agents running on node %s is %v",
		nodeName,
		len(ciliumAgentPodsOnSameNode.Items),
	)
	switch {
	case len(ciliumAgentPodsOnSameNode.Items) == 0:
		return "", false, fmt.Errorf(
			"[SafeAgentUpdater] There aren't agent pods on node %s",
			nodeName,
		)
	case len(ciliumAgentPodsOnSameNode.Items) > 1:
		return "", false, fmt.Errorf(
			"[SafeAgentUpdater] There are more than one running agent pods on node %s",
			nodeName,
		)
	}

	currentPod := ciliumAgentPodsOnSameNode.Items[0]
	log.Infof(
		"[SafeAgentUpdater] Name of pod which running on the same node is %s",
		currentPod.Name,
	)
	currentAgentGeneration := currentPod.Annotations[generationAnnotation]
	log.Infof(
		"[SafeAgentUpdater] Generation of pod %s is %s",
		currentPod.Name,
		currentAgentGeneration,
	)

	if desiredAgentGeneration == currentAgentGeneration {
		log.Infof(
			"[SafeAgentUpdater] Desired agent generation(%s) and current(%s) are same. Nothing to do.",
			desiredAgentGeneration,
			currentAgentGeneration,
		)
		return currentPod.Name, true, nil
	}
	log.Infof(
		"[SafeAgentUpdater] Desired agent generation(%s) and current(%s) are not the same. Reconsile is needed",
		desiredAgentGeneration,
		currentAgentGeneration,
	)
	return currentPod.Name, false, nil

}

func deletePod(kubeClient kubernetes.Interface, podName string) error {
	err := kubeClient.CoreV1().Pods(ciliumNS).Delete(
		context.TODO(),
		podName,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf(
			"[SafeAgentUpdater] Failed to delete pod %s. Error: %v",
			podName,
			err,
		)
	}
	log.Infof(
		"[SafeAgentUpdater] Pod %s/%s deleted",
		ciliumNS,
		podName,
	)
	return nil
}

func waitUntilNewPodCreatedAndBecomeReady(kubeClient kubernetes.Interface, nodeName string, scanIterations int) error {
	var newPodName string
	for i := 0; i < scanIterations; i++ {
		log.Infof("[SafeAgentUpdater] Waiting until new pod created on same node")
		ciliumAgentPodsOnSameNode, err := kubeClient.CoreV1().Pods(ciliumNS).List(
			context.TODO(),
			metav1.ListOptions{
				LabelSelector: "app=agent",
				FieldSelector: "spec.nodeName=" + nodeName,
			},
		)
		if err != nil {
			log.Errorf("[SafeAgentUpdater] Failed to list pods on same node. Error: %v.", err)
		}
		log.Infof(
			"[SafeAgentUpdater] Count of agents running on node %s is %v",
			nodeName,
			len(ciliumAgentPodsOnSameNode.Items),
		)

		if len(ciliumAgentPodsOnSameNode.Items) == 1 &&
			ciliumAgentPodsOnSameNode.Items[0].DeletionTimestamp == nil {

			newPodName = ciliumAgentPodsOnSameNode.Items[0].Name
			log.Infof(
				"[SafeAgentUpdater] New pod created with name %s",
				newPodName,
			)
			break
		} else if i == scanIterations-1 {
			return fmt.Errorf("[SafeAgentUpdater] Failed to get one new pod after %v attempts", scanIterations)
		}
		time.Sleep(scanInterval)
	}
	for i := 0; i < scanIterations; i++ {
		newPod, err := kubeClient.CoreV1().Pods(ciliumNS).Get(
			context.TODO(),
			newPodName,
			metav1.GetOptions{},
		)
		if err != nil {
			log.Errorf("[SafeAgentUpdater] Failed to get pod %s. Error: %v.", newPodName, err)
		}
		log.Infof("[SafeAgentUpdater] Waiting until new pod %s become Ready", newPod.Name)

		if isPodReady(newPod) {
			log.Infof("[SafeAgentUpdater] Pod %s id Ready", newPod.Name)
			break
		} else if i == scanIterations-1 {
			return fmt.Errorf(
				"[SafeAgentUpdater] Failed to wait until new pod %s become Ready after %v attempts",
				newPod.Name,
				scanIterations,
			)
		}
		time.Sleep(scanInterval)
	}
	log.Infof("[SafeAgentUpdater] Cilium agent on node %s successfully reloaded", nodeName)
	return nil
}

func isPodReady(pod *v1.Pod) bool {
	var isReady = false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			isReady = true
			break
		}
	}
	return isReady
}
