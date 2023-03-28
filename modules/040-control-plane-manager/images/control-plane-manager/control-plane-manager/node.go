package main

import (
	"context"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func annotateNode() error {
	log.Infof("annotate node %s with annotation %s", nodeName, waitingApprovalAnnotation)
	node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if _, ok := node.Annotations[approvedAnnotation]; ok {
		// node already approved, no need to annotate
		log.Infof("node %s already approved by annotation %s, no need to annotate", nodeName, approvedAnnotation)
		return nil
	}

	node.Annotations[waitingApprovalAnnotation] = ""

	_, err = k8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func waitNodeApproval() error {
	for i := 0; i < maxRetries; i++ {
		log.Infof("waiting for %s annotation on our node %s", approvedAnnotation, nodeName)
		node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, ok := node.Annotations[approvedAnnotation]; ok {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return errors.Errorf("can't get annotation %s from our node %s", approvedAnnotation, nodeName)
}
