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
	"bytes"
	"context"
	"fmt"
	"golang.org/x/mod/semver"
	"os"
	"os/exec"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	cniCiliumBinaryPath          = "/hostbin/cilium-cni"
	ciliumNS                     = "d8-cni-cilium"
	generationAnnotation         = "safe-agent-updater-daemonset-generation"
	migrationSucceededAnnotation = "network.deckhouse.io/cilium-1-17-migration-succeeded"
	migrationRequiredAnnotation  = "network.deckhouse.io/cilium-1-17-migration-disruptive-update-required"
	scanInterval                 = 3 * time.Second
	scanIterations               = 20
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
	desiredAgentImageHash := os.Getenv("CILIUM_AGENT_DESIRED_IMAGE_HASH")
	if len(desiredAgentImageHash) == 0 {
		log.Fatalf("[SafeAgentUpdater] Failed to get env CILIUM_AGENT_DESIRED_IMAGE_HASH.")
	}

	currentAgentPodName, currentAgentImageHash, isCurrentAgentPodGenerationDesired, err := checkAgentPodGeneration(kubeClient, nodeName)
	if err != nil {
		log.Fatal(err)
	}
	if !isCurrentAgentPodGenerationDesired {
		if isMigrationSucceeded(kubeClient, nodeName) {
			log.Infof("[SafeAgentUpdater] The 1.17-migration-disruptive-update already succeeded")
			err = setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false")
			if err != nil {
				log.Fatal(err)
			}
		} else if isCurrentImageEqUpcoming(desiredAgentImageHash, currentAgentImageHash) {
			log.Infof("[SafeAgentUpdater] The current agent image is the same as in the upcoming update, so the 1.17-migration-disruptive-update is no needed.")
			err = setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false")
			if err != nil {
				log.Fatal(err)
			}
		} else if !isCiliumExistOnNode() {
			log.Infof("[SafeAgentUpdater] Cilium CNI binary does not exist on node %s.", nodeName)
			if err := setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false"); err != nil {
				log.Fatal(err)
			}
		} else if ok, version := isCiliumCNIVersionAlreadyUpToDate(); ok {
			log.Infof("[SafeAgentUpdater] Cilium CNI plugin version is not less than 1.17: %s", version)
			if err := setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false"); err != nil {
				log.Fatal(err)
			}
		} else if areSTSPodsPresentOnNode(kubeClient, nodeName) {
			log.Infof("[SafeAgentUpdater] The current agent image is not the same as in the upcoming update, and sts pods are present on node, so the 1.17-migration-disruptive-update is needed")
			err = setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "true")
			if err != nil {
				log.Fatal(err)
			}
			err = waitUntilDisruptionApproved(kubeClient, nodeName)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Infof("[SafeAgentUpdater] The current agent image is not the same as in the upcoming update, but sts pods are not present on node, so the 1.17-migration-disruptive-update is no needed")
			if err := setAnnotationToNode(kubeClient, nodeName, migrationSucceededAnnotation, ""); err != nil {
				log.Fatal(err)
			}
			err = setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false")
			if err != nil {
				log.Fatal(err)
			}
		}
		err = deletePod(kubeClient, currentAgentPodName)
		if err != nil {
			log.Fatal(err)
		}
		err := waitUntilNewPodCreatedAndBecomeReady(kubeClient, nodeName, scanIterations)
		if err != nil {
			log.Fatal(err)
		}
		err = setAnnotationToNode(kubeClient, nodeName, migrationSucceededAnnotation, "")
		if err != nil {
			log.Fatal(err)
		}
	}
	err = setAnnotationToNode(kubeClient, nodeName, migrationRequiredAnnotation, "false")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("[SafeAgentUpdater] Finished and exit")
}

func isCiliumExistOnNode() bool {
	_, err := os.Stat(cniCiliumBinaryPath)
	return !os.IsNotExist(err)
}

func isCiliumCNIVersionAlreadyUpToDate() (bool, string) {
	cmd := exec.Command(cniCiliumBinaryPath, "VERSION")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("[SafeAgentUpdater] Failed to execute cilium-cni binary: %v, stderr: %s", err, stderr.String())
		return false, ""
	}

	version := regexp.MustCompile(`\d+\.\d+\.\d+`).FindString(stderr.String())
	if version == "" {
		log.Fatalf("[SafeAgentUpdater] Failed to parse cilium-cni version")
		return false, ""
	}

	return semver.Compare("v"+version, "v1.17.0") >= 0, version
}

func checkAgentPodGeneration(kubeClient kubernetes.Interface, nodeName string) (currentAgentPodName string, currentAgentImageHash string, isCurrentAgentPodGenerationDesired bool, err error) {
	ciliumAgentDS, err := kubeClient.AppsV1().DaemonSets(ciliumNS).Get(
		context.TODO(),
		"agent",
		metav1.GetOptions{},
	)
	if err != nil {
		return "", "", false, fmt.Errorf(
			"[SafeAgentUpdater] Failed to get DaemonSets %s/agent. Error: %v",
			ciliumNS,
			err,
		)
	}

	desiredAgentGeneration := ciliumAgentDS.Spec.Template.Annotations[generationAnnotation]
	if len(desiredAgentGeneration) == 0 {
		return "", "", false, fmt.Errorf(
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
		return "", "", false, fmt.Errorf(
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
		return "", "", false, fmt.Errorf(
			"[SafeAgentUpdater] There aren't agent pods on node %s",
			nodeName,
		)
	case len(ciliumAgentPodsOnSameNode.Items) > 1:
		return "", "", false, fmt.Errorf(
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
		return currentPod.Name, currentPod.Spec.Containers[0].Image, true, nil
	}
	log.Infof(
		"[SafeAgentUpdater] Desired agent generation(%s) and current(%s) are not the same. Reconsile is needed",
		desiredAgentGeneration,
		currentAgentGeneration,
	)
	return currentPod.Name, currentPod.Spec.Containers[0].Image, false, nil

}

func isMigrationSucceeded(kubeClient kubernetes.Interface, nodeName string) bool {
	node, err := kubeClient.CoreV1().Nodes().Get(
		context.TODO(),
		nodeName,
		metav1.GetOptions{},
	)
	if err != nil {
		log.Errorf("[SafeAgentUpdater] Failed to get node %s. Error: %v.", nodeName, err)
		return false
	}
	if val, ok := node.Annotations[migrationSucceededAnnotation]; ok && val == "" {
		return true
	}
	return false
}

func isCurrentImageEqUpcoming(desiredAgentImageHash, currentAgentImageHash string) bool {
	return desiredAgentImageHash == currentAgentImageHash
}

func areSTSPodsPresentOnNode(kubeClient kubernetes.Interface, nodeName string) bool {
	allPodsOnNode, err := kubeClient.CoreV1().Pods("").List(
		context.TODO(),
		metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + nodeName,
		},
	)
	if err != nil {
		log.Errorf("[SafeAgentUpdater] Failed to list pods on same node. Error: %v.", err)
		return false
	}
	for _, pod := range allPodsOnNode.Items {
		for _, ownerRef := range pod.OwnerReferences {
			if ownerRef.Kind == "StatefulSet" {
				return true
			}
		}
	}
	return false
}

func setAnnotationToNode(kubeClient kubernetes.Interface, nodeName string, annotationKey string, annotationValue string) error {
	node, err := kubeClient.CoreV1().Nodes().Get(
		context.TODO(),
		nodeName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("[SafeAgentUpdater] Failed to get node %s. Error: %v", nodeName, err)
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annotationKey] = annotationValue
	_, err = kubeClient.CoreV1().Nodes().Update(
		context.TODO(),
		node,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("[SafeAgentUpdater] Failed to update node %s. Error: %v", nodeName, err)
	}
	return nil
}

func waitUntilDisruptionApproved(kubeClient kubernetes.Interface, nodeName string) error {
	for {
		node, err := kubeClient.CoreV1().Nodes().Get(
			context.TODO(),
			nodeName,
			metav1.GetOptions{},
		)
		if err != nil {
			log.Errorf("[SafeAgentUpdater] Failed to get node %s. Error: %v", nodeName, err)
		} else if val, ok := node.Annotations["update.node.deckhouse.io/disruption-approved"]; ok && val == "" {
			return nil
		}
		log.Infof("[SafeAgentUpdater] Waiting until disruption update on node %s was approved", nodeName)
		time.Sleep(10 * time.Second)
	}
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
