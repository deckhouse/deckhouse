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
	"log/slog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func annotateNode() error {
	log.Info("phase: annotate node", slog.String("node", config.NodeName), slog.String("annotation", waitingApprovalAnnotation))
	node, err := config.K8sClient.CoreV1().Nodes().Get(context.TODO(), config.NodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if _, ok := node.Annotations[approvedAnnotation]; ok {
		// node already approved, no need to annotate
		log.Info("node already approved by annotation, no need to annotate", slog.String("node", config.NodeName), slog.String("annotation", approvedAnnotation))
		return nil
	}

	node.Annotations[waitingApprovalAnnotation] = ""

	_, err = config.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func waitNodeApproval() error {
	log.Info("phase: waiting node approval with annotation", slog.String("node", config.NodeName), slog.String("annotation", approvedAnnotation))

	log.Info("waiting for annotation on our node", slog.String("node", config.NodeName), slog.String("annotation", approvedAnnotation))
	node, err := config.K8sClient.CoreV1().Nodes().Get(context.TODO(), config.NodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if _, ok := node.Annotations[approvedAnnotation]; ok {
		return nil
	}

	return fmt.Errorf("can't get annotation %s from our node %s", approvedAnnotation, config.NodeName)
}
