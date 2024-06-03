/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package info

import (
	"fmt"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
)

type MastersInfo struct {
	logger *logrus.Entry
}

func NewMastersInfo(logger *logrus.Entry) MastersInfo {
	return MastersInfo{
		logger: logger,
	}
}

func (m *MastersInfo) MastersInfoWaitByFunc(cmpFunc func(nodes *corev1.NodeList) bool) (*corev1.NodeList, error) {
	nodes, isReady, err := kube_actions.WaitMasterNodesInfo(cmpFunc)
	if err != nil {
		return nil, err
	}
	if !isReady {
		return nil, fmt.Errorf("Service is not ready yet")
	}
	return nodes, err
}

func (m *MastersInfo) MastersInfoGet() (*corev1.NodeList, error) {
	return kube_actions.GetMasterNodesInfo()
}
