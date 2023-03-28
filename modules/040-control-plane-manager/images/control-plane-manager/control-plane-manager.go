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

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	// get k8s dynamic client
	k8sClient, err := newClient()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// get hostname
	node, err := getNodeName()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	err = annotateNode(k8sClient, node)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	err = waitNodeApproval(k8sClient, node)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
