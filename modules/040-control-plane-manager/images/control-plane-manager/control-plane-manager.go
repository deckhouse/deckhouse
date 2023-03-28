/*
Copyright 2023 Flant JSC

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

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	waitingApprovalAnnotation = `control-plane-manager.deckhouse.io/waiting-for-approval`
	approvedAnnotation        = `control-plane-manager.deckhouse.io/approved`
	maxRetries                = 42
	namespace                 = `kube-system`
	minimalKubernetesVersion = `1.22`
	maximalKubernetesVersion = `1.26`
)

func newClient() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func getNodeName() (string, error) {
	return os.Hostname()
}

func annotateNode(k8sClient *kubernetes.Clientset, nodeName string) error {
	log.Infof("annotate node %s with annotation '%s'", nodeName, waitingApprovalAnnotation)
	node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if _, ok := node.Annotations[approvedAnnotation]; ok {
		// node already approved, no need to annotate
		log.Infof("node %s already approved by annotation '%s', no need to annotate", nodeName, approvedAnnotation)
		return nil
	}

	node.Annotations[waitingApprovalAnnotation] = ""

	_, err = k8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func waitNodeApproval(k8sClient *kubernetes.Clientset, nodeName string) error {
	for i := 0; i < maxRetries; i++ {
		log.Infof("waiting for '%s' annotation on our node %s", approvedAnnotation, nodeName)
		node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, ok := node.Annotations[approvedAnnotation]; ok {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return errors.Errorf("can't get annotation '%s' from our node %s", approvedAnnotation, nodeName)
}

func waitImageHolderContainers(k8sClient *kubernetes.Clientset, podName string) error {
	for {
		log.Info("waiting for all image-holder containers will be ready")
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		isReady := true
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == "control-plane-manager" {
				continue
			}
			if !container.Ready {
				isReady = false
				break
			}
		}

		if isReady {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
}

func kubernetesVersionAllowed(version string) bool {
	minimalConstraint, err := semver.NewConstraint(">= " + minimalKubernetesVersion)
	if err != nil {
		log.Fatal(err)
	}

	maximalConstraint, err := semver.NewConstraint("< " + maximalKubernetesVersion)
	if err != nil {
		log.Fatal(err)
	}

	v := semver.MustParse(version)
	return minimalConstraint.Check(v) && maximalConstraint.Check(v)
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	pod := os.Getenv("MY_POD_NAME")
	if pod == "" {
		log.Fatal("MY_POD_NAME env should be set")
	}

	k8s := os.Getenv("KUBERNETES_VERSION")
	if k8s == "" {
		log.Fatal("KUBERNETES_VERSION env should be set")
	}

	// check kubernetes version
	if !kubernetesVersionAllowed(k8s) {
		log.Fatal("kubernetes version %s is not allowed", k8s)
	}

	// get hostname
	node, err := getNodeName()
	if err != nil {
		log.Fatal(err)
	}

	// get k8s dynamic client
	k8sClient, err := newClient()
	if err != nil {
		log.Fatal(err)
	}

	err = annotateNode(k8sClient, node)
	if err != nil {
		log.Fatal(err)
	}

	err = waitNodeApproval(k8sClient, node)
	if err != nil {
		log.Fatal(err)
	}

	err = waitImageHolderContainers(k8sClient, pod)
	if err != nil {
		log.Fatal(err)
	}

}
