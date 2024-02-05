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
	"os"
	"time"

	v1 "k8s.io/api/core/v1"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ciliumNS                     = "d8-cni-cilium"
	generationChecksumAnnotation = "safe-agent-updater-daemonset-generation"
	scanInterval                 = 3 * time.Second
)

func main() {
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[SafeAgentUpdater] Failed to init kubeClient. Error: %v", err)
	}

	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		log.Fatalf("[SafeAgentUpdater] Failed to get NODE_NAME.")
	}

	ciliumAgentDS, err := kubeClient.AppsV1().DaemonSets(ciliumNS).Get(
		context.TODO(),
		"agent",
		metav1.GetOptions{},
	)
	if err != nil {
		log.Fatalf("[SafeAgentUpdater] Failed to get DaemonSets %s/agent. Error: %v", ciliumNS, err)
	}

	ciliumAgentDSGenerationChecksum := ciliumAgentDS.Spec.Template.Annotations[generationChecksumAnnotation]
	if len(ciliumAgentDSGenerationChecksum) == 0 {
		log.Fatalf(
			"[SafeAgentUpdater] DaemonSets %s/agent doesn't have annotations %s.",
			ciliumNS,
			generationChecksumAnnotation,
		)
	}
	log.Infof(
		"[SafeAgentUpdater] Current generation of DS %s/agent is %s",
		ciliumNS,
		ciliumAgentDSGenerationChecksum,
	)

	ciliumAgentPodsOnSameNode, err := kubeClient.CoreV1().Pods(ciliumNS).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "app=agent",
			FieldSelector: "spec.nodeName=" + nodeName,
		},
	)
	if err != nil {
		log.Fatalf("[SafeAgentUpdater] Failed to list pods on same node. Error: %v", err)
	}
	log.Infof(
		"[SafeAgentUpdater] Count of agents running on node %s is %v",
		nodeName,
		len(ciliumAgentPodsOnSameNode.Items),
	)
	switch {
	case len(ciliumAgentPodsOnSameNode.Items) == 0:
		log.Fatalf("[SafeAgentUpdater] There aren't agent pods on node %s", nodeName)
	case len(ciliumAgentPodsOnSameNode.Items) > 1:
		log.Fatalf("[SafeAgentUpdater] There are more than one running agent pods on node %s", nodeName)
	}
	currentPod := ciliumAgentPodsOnSameNode.Items[0]
	log.Infof(
		"[SafeAgentUpdater] Name of pod which running on the same node is %s",
		currentPod.Name,
	)
	currentPodGenerationChecksum := currentPod.Annotations[generationChecksumAnnotation]
	log.Infof(
		"[SafeAgentUpdater] Generation of pod %s is %s",
		currentPod.Name,
		currentPodGenerationChecksum,
	)
	if ciliumAgentDSGenerationChecksum == currentPodGenerationChecksum {
		log.Infof("[SafeAgentUpdater] The cilium agent pod completely matches its DaemonSet. Nothing to do")
	}
	if ciliumAgentDSGenerationChecksum != currentPodGenerationChecksum {
		log.Infof(
			"[SafeAgentUpdater] Generation on DS(%s) and Pod(%s) are not the same. Deleting Pod %s",
			ciliumAgentDSGenerationChecksum,
			currentPodGenerationChecksum,
			currentPod.Name,
		)
		err := kubeClient.CoreV1().Pods(ciliumNS).Delete(
			context.TODO(),
			currentPod.Name,
			metav1.DeleteOptions{},
		)
		if err != nil {
			log.Fatalf("[SafeAgentUpdater] Failed to delete pod %s. Error: %v", currentPod.Name, err)
		}
		log.Infof(
			"[SafeAgentUpdater] Pod %s/%s deleted",
			ciliumNS,
			currentPod.Name,
		)
		var newPodName string
		for {
			log.Infof("[SafeAgentUpdater] Waiting until new pod created on same node")
			ciliumAgentPodsOnSameNode, err = kubeClient.CoreV1().Pods(ciliumNS).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: "app=agent",
					FieldSelector: "spec.nodeName=" + nodeName,
				},
			)
			if err != nil {
				log.Errorf("[SafeAgentUpdater] Failed to list pods on same node. Error: %v", err)
			}
			log.Infof(
				"[SafeAgentUpdater] Count of agents running on node %s is %v",
				nodeName,
				len(ciliumAgentPodsOnSameNode.Items),
			)

			if len(ciliumAgentPodsOnSameNode.Items) == 1 {
				newPodName = ciliumAgentPodsOnSameNode.Items[0].Name
				log.Infof(
					"[SafeAgentUpdater] New pod created with name %s",
					newPodName,
				)
				break
			}
			time.Sleep(scanInterval)
		}
		for {
			newPod, err := kubeClient.CoreV1().Pods(ciliumNS).Get(
				context.TODO(),
				newPodName,
				metav1.GetOptions{},
			)
			if err != nil {
				log.Errorf("[SafeAgentUpdater] Failed to get pod %s. Error: %v", newPodName, err)
			}
			log.Infof("[SafeAgentUpdater] Waiting until new pod %s become Ready", newPod.Name)

			if isPodReady(newPod) {
				break
			}
			time.Sleep(scanInterval)
		}
		log.Infof("[SafeAgentUpdater] Cilium agent on node %s successfully reloaded", nodeName)
	}
	log.Infof("[SafeAgentUpdater] Finished and exit")
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
