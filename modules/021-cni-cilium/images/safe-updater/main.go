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

	corev1 "k8s.io/api/core/v1"

	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ciliumNs                      = "d8-cni-cilium"
	generationChecksumAnnotations = "daemonset-generation.deckhouse.io/checksum"
	scanInterval                  = 3
)

var (
	NewPodName string
)

func main() {
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err)
	}

	SelfNodeName := os.Getenv("NODE_NAME")

	CiliumAgentDS, err := kubeClient.AppsV1().DaemonSets(ciliumNs).Get(
		context.TODO(),
		"agent",
		metav1.GetOptions{},
	)
	if err != nil {
		log.Error(err)
	}
	CiliumAgentDSGenerationChecksum := CiliumAgentDS.Spec.Template.Annotations[generationChecksumAnnotations]
	log.Infof(
		"[SafeUpdater] Current generation of DS %s/agent is %s",
		ciliumNs,
		CiliumAgentDSGenerationChecksum,
	)

	CiliumAgentPodsOnSameNode, err := kubeClient.CoreV1().Pods(ciliumNs).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: "app=agent",
			FieldSelector: "spec.nodeName=" + SelfNodeName,
		},
	)
	if err != nil {
		log.Error(err)
	}
	log.Infof(
		"[SafeUpdater] Count of agents running on node %s is %v",
		SelfNodeName,
		len(CiliumAgentPodsOnSameNode.Items),
	)
	switch {
	case len(CiliumAgentPodsOnSameNode.Items) == 0:
		log.Errorf("On node %s no one pod of agent", SelfNodeName)
	case len(CiliumAgentPodsOnSameNode.Items) > 1:
		log.Errorf("On node %s running more then one agent", SelfNodeName)
	}
	CurrentPod := CiliumAgentPodsOnSameNode.Items[0]
	log.Infof(
		"[SafeUpdater] Name of pod which running on the same node is %s",
		CurrentPod.Name,
	)
	CurrentPodGenerationChecksum := CurrentPod.Annotations[generationChecksumAnnotations]
	log.Infof(
		"[SafeUpdater] Generation of pod %s is %s",
		CurrentPod.Name,
		CurrentPodGenerationChecksum,
	)
	if CiliumAgentDSGenerationChecksum != CurrentPodGenerationChecksum {
		log.Infof(
			"[SafeUpdater] Generation on DS(%s) and Pod(%s) are not the same. Deleting Pod %s",
			CiliumAgentDSGenerationChecksum,
			CurrentPodGenerationChecksum,
			CurrentPod.Name,
		)
		err := kubeClient.CoreV1().Pods(ciliumNs).Delete(
			context.TODO(),
			CurrentPod.Name,
			metav1.DeleteOptions{},
		)
		if err != nil {
			log.Error(err)
		}
		log.Infof(
			"[SafeUpdater] Pod %s/%s deleted",
			ciliumNs,
			CurrentPod.Name,
		)

		for {
			log.Infof("[SafeUpdater] Waiting until new pod created on same node")
			CiliumAgentPodsOnSameNode, err = kubeClient.CoreV1().Pods(ciliumNs).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: "app=agent",
					FieldSelector: "spec.nodeName=" + SelfNodeName,
				},
			)
			if err != nil {
				log.Error(err)
			}
			log.Infof(
				"[SafeUpdater] Count of agents running on node %s is %v",
				SelfNodeName,
				len(CiliumAgentPodsOnSameNode.Items),
			)
			if len(CiliumAgentPodsOnSameNode.Items) == 1 &&
				CiliumAgentPodsOnSameNode.Items[0].Name != "" {
				NewPodName = CiliumAgentPodsOnSameNode.Items[0].Name
				log.Infof(
					"[SafeUpdater] New pod created with name %s",
					NewPodName,
				)
				break
			}
			time.Sleep(scanInterval * time.Second)
		}

		for {
			NewPod, err := kubeClient.CoreV1().Pods(ciliumNs).Get(
				context.TODO(),
				NewPodName,
				metav1.GetOptions{},
			)
			if err != nil {
				log.Error(err)
			}
			log.Infof("[SafeUpdater] Waiting until new pod %s become Ready", NewPod.Name)

			for _, cond := range NewPod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					break
				}
			}
			time.Sleep(scanInterval * time.Second)
		}
		log.Infof("[SafeUpdater] Cilium agent on node %s successfully reloaded", SelfNodeName)
	} else {
		log.Infof("[SafeUpdater] The cilium agent pod completely matches its DaemonSet. Nothing to do")
	}
	log.Infof("[SafeUpdater] Finished and exit")
}
