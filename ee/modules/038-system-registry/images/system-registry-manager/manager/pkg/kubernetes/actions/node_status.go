/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"system-registry-manager/internal/config"
)

type NodeStatus struct {
	FromMe      ActionStatus
	FromHandler ActionStatus
}

func GetNodeStatus() (NodeStatus, error) {
	cfg := config.GetConfig()
	log.Infof("Fetching node status for node: %s", cfg.HostName)
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error fetching node status: %v", err)
		return NodeStatus{}, err
	}
	return GetNodeStatusFromAnnotations(node.Annotations)
}

func GetNodeStatusFromAnnotations(annotations map[string]string) (NodeStatus, error) {
	nodeStatus := NodeStatus{}
	var err error

	log.Debug("Getting node status from annotations")
	// From me
	if annotationValue, ok := annotations[config.AnnotationFromMe]; ok {
		nodeStatus.FromMe, err = fromStringToStat(strings.TrimSpace(annotationValue))
		if err != nil {
			log.Errorf("Error parsing '%s' annotation: %v", config.AnnotationFromMe, err)
			return nodeStatus, err
		}
	}

	// From handler
	if annotationValue, ok := annotations[config.AnnotationFromHandler]; ok {
		nodeStatus.FromHandler, err = fromStringToStat(strings.TrimSpace(annotationValue))
		if err != nil {
			log.Errorf("Error parsing '%s' annotation: %v", config.AnnotationFromHandler, err)
			return nodeStatus, err
		}
	}
	return nodeStatus, nil
}

func WaitNodeStatus(cmpFunc func(nodeStatus *NodeStatus) bool) error {
	for i := 0; i < config.MaxRetries; i++ {
		log.Debugf("Checking node status, attempt %d/%d", i+1, config.MaxRetries)
		nodeStatus, err := GetNodeStatus()
		if err != nil {
			log.Errorf("Error getting node status: %v", err)
			return err
		}
		if cmpFunc(&nodeStatus) {
			log.Info("Node status condition met")
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
	log.Infof("Setting status for node: %s", cfg.HostName)
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting node: %v", err)
		return err
	}

	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		log.Errorf("Error getting node status from annotations: %v", err)
		return err
	}

	// If the same and not completed - nothing to do
	if ActionStatusEqual(&newStatus, &nodeStatus.FromMe) && !nodeStatus.FromMe.Completed {
		log.Info("Status is the same and not completed")
	} else {
		// Update status
		node.Annotations[config.AnnotationFromMe], err = newStatus.toString()
		if err != nil {
			log.Errorf("Error converting new status to string: %v", err)
			return err
		}

		_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			log.Errorf("Error updating node status: %v", err)
			return err
		}
	}

	// Wait for approval
	log.Info("Waiting for approval...")
	cmpFunc := func(nodeStatus *NodeStatus) bool {
		if nodeStatus == nil {
			return false
		}
		return nodeStatus.FromMe.Approved
	}

	err = WaitNodeStatus(cmpFunc)
	if err != nil {
		log.Errorf("Error waiting for approval: %v", err)
		return err
	}
	log.Info("Approval received.")
	return nil
}

func SetMyStatusDone() error {
	log.Info("Setting my status to done")
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting node: %v", err)
		return err
	}

	// Get node status from annotations
	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		log.Errorf("Error getting node status from annotations: %v", err)
		return err
	}

	// If Completed - nothing to do
	if nodeStatus.FromMe.Completed {
		log.Info("Status already completed, no action needed")
		return nil
	}

	// else - change to true
	nodeStatus.FromMe.Completed = true
	node.Annotations[config.AnnotationFromMe], err = nodeStatus.FromMe.toString()
	if err != nil {
		log.Errorf("Error converting completed status to string: %v", err)
		return err
	}

	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating node status: %v", err)
		return err
	}
	log.Info("Status set to done.")
	return nil
}

func ClearMyStatus() error {
	log.Info("Clearing my status")
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting node: %v", err)
		return err
	}

	// If empty - nothing to do
	if node.Annotations[config.AnnotationFromMe] == "" {
		log.Info("Status is already clear, no action needed")
		return nil
	}

	// else - clear
	node.Annotations[config.AnnotationFromMe] = ""
	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error clearing status: %v", err)
		return err
	}
	log.Info("Status cleared.")
	return nil
}

func ApproveHandlerStatus() error {
	log.Info("Approving handler status")
	// Get annotations
	cfg := config.GetConfig()
	node, err := cfg.K8sClient.CoreV1().Nodes().Get(context.TODO(), cfg.HostName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting node: %v", err)
		return err
	}

	// Get node status from annotations
	nodeStatus, err := GetNodeStatusFromAnnotations(node.Annotations)
	if err != nil {
		log.Errorf("Error getting node status from annotations: %v", err)
		return err
	}

	// If approved - nothing to do
	if nodeStatus.FromHandler.Approved {
		log.Info("Handler status already approved, no action needed")
		return nil
	}

	// else - change to true
	nodeStatus.FromHandler.Approved = true
	node.Annotations[config.AnnotationFromHandler], err = nodeStatus.FromHandler.toString()
	if err != nil {
		log.Errorf("Error converting approved status to string: %v", err)
		return err
	}

	_, err = cfg.K8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		log.Errorf("Error updating handler status: %v", err)
		return err
	}
	log.Info("Handler status approved.")
	return nil
}
