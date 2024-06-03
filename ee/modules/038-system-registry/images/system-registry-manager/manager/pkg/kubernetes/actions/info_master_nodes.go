/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package actions

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

const (
	masterNodeLabel   = "node-role.kubernetes.io/master="
	controlPlaneLabel = "node-role.kubernetes.io/control-plane="
)

func GetNodesInfoByLabels(labelSelector string) (*corev1.NodeList, error) {
	cfg := pkg_cfg.GetConfig()

	nodes, err := cfg.K8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting NodeList: %v", err)
	}
	return nodes, nil
}

func GetMasterNodesInfo() (*corev1.NodeList, error) {
	nodeList, err := GetNodesInfoByLabels(controlPlaneLabel)

	if err != nil || nodeList != nil {
		return nodeList, err
	}

	return GetNodesInfoByLabels(masterNodeLabel)
}

func WaitMasterNodesInfo(cmpFunc func(nodes *corev1.NodeList) bool) (*corev1.NodeList, bool, error) {
	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		nodeList, err := GetMasterNodesInfo()
		if err != nil {
			return nil, false, err
		}
		if cmpFunc(nodeList) {
			return nodeList, true, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, false, nil
}
