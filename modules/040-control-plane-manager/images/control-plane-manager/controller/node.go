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
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func annotateNode() error {
	log.Infof("phase: annotate node %s with annotation %s", config.NodeName, waitingApprovalAnnotation)
	node, err := config.K8sClient.CoreV1().Nodes().Get(context.TODO(), config.NodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if _, ok := node.Annotations[approvedAnnotation]; ok {
		// node already approved, no need to annotate
		log.Infof("node %s already approved by annotation %s, no need to annotate", config.NodeName, approvedAnnotation)
		return nil
	}

	node.Annotations[waitingApprovalAnnotation] = ""

	_, err = config.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func waitNodeApproval() error {
	log.Infof("phase: waiting node node %s approval with annotation %s", config.NodeName, approvedAnnotation)

	for i := 0; i < maxRetries; i++ {
		log.Infof("waiting for %s annotation on our node %s", approvedAnnotation, config.NodeName)
		node, err := config.K8sClient.CoreV1().Nodes().Get(context.TODO(), config.NodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, ok := node.Annotations[approvedAnnotation]; ok {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("can't get annotation %s from our node %s", approvedAnnotation, config.NodeName)
}
