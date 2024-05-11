/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubeapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"system-registry-manager/internal/config"
)

type NodeStatus struct {
	FromMe      ActionStatus
	FromHandler ActionStatus
}

func GetNodeStatus() (NodeStatus, error) {
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		return NodeStatus{}, err
	}
	return GetNodeStatusFromAnnotations(node.Annotations)
}

func GetNodeStatusFromAnnotations(annotations map[string]string) (NodeStatus, error) {
	nodeStatus := NodeStatus{}
	var err error

	// From me
	if annotationValue, ok := annotations[config.AnnotationFromMe]; ok {
		nodeStatus.FromMe, err = fromStringToStat(strings.TrimSpace(annotationValue))
		if err != nil {
			return nodeStatus, err
		}
	}

	// From handler
	if annotationValue, ok := annotations[config.AnnotationFromHandler]; ok {
		nodeStatus.FromHandler, err = fromStringToStat(strings.TrimSpace(annotationValue))
		if err != nil {
			return nodeStatus, err
		}
	}
	return nodeStatus, nil
}

func WaitNodeStatus(cmpFunc func(nodeStatus *NodeStatus) bool) error {
	for i := 0; i < config.MaxRetries; i++ {
		nodeStatus, err := GetNodeStatus()
		if err != nil {
			return err
		}
		if cmpFunc(&nodeStatus) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("reached maximum retries")
}

func SetMyStatusAndWaitApprove(actionName string, actionPriority int) error {
	cfg := config.GetConfig()

	// Prepare new status
	newStatus := ActionStatus{
		Name:      actionName,
		Priority:  actionPriority,
		Approved:  false,
		Completed: false,
	}

	// Get current status
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		return err
	}

	// If the same and not completed - nothing to do
	if ActionStatusEqual(&newStatus, &nodeStatus.FromMe) && !nodeStatus.FromMe.Completed {
		return nil
	}

	// Update status
	node.Annotations[config.AnnotationFromMe], err = newStatus.toString()
	if err != nil {
		return err
	}

	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// Wait for approval
	cmpFunc := func(nodeStatus *NodeStatus) bool {
		if nodeStatus == nil {
			return false
		}
		return nodeStatus.FromMe.Approved
	}

	// Add logging
	fmt.Println("Waiting for approval...")
	err = WaitNodeStatus(cmpFunc)
	if err != nil {
		fmt.Println("Error waiting for approval:", err)
		return err
	}
	fmt.Println("Approval received.")
	return nil
}
func SetMyStatusDone() error {
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Get node status from annotations
	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		return err
	}

	// If Completed - nofing to do
	if nodeStatus.FromMe.Completed {
		return nil
	}

	// else - change to true
	nodeStatus.FromMe.Completed = true
	node.Annotations[config.AnnotationFromMe], err = nodeStatus.FromMe.toString()
	if err != nil {
		return err
	}

	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func ClearMyStatus() error {
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// If empty - nothing to do
	if node.Annotations[config.AnnotationFromMe] == "" {
		return nil
	}

	// else - clear
	node.Annotations[config.AnnotationFromMe] = ""
	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func ApproveHandlerStatus() error {
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Get node status from annotations
	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		return err
	}

	// If approved - nofing to do
	if nodeStatus.FromHandler.Approved {
		return nil
	}

	// else - change to true
	nodeStatus.FromHandler.Approved = true
	node.Annotations[config.AnnotationFromHandler], err = nodeStatus.FromHandler.toString()
	if err != nil {
		return err
	}

	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
