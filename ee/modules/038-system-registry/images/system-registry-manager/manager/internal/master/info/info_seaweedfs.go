/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package info

import (
	"fmt"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
	seaweedfs_client "system-registry-manager/pkg/seaweedfs/client"
)

type SeaweedfsNodeInfo struct {
	logger *logrus.Entry
	Pod    corev1.Pod
}

func (s *SeaweedfsNodeInfo) CreateClient() (*seaweedfs_client.Client, error) {
	masterAddress := fmt.Sprintf("%s:%d", s.Pod.Status.PodIP, pkg_cfg.SeaweedfsMasterPort)
	filerAddress := fmt.Sprintf("%s:%d", s.Pod.Status.PodIP, pkg_cfg.SeaweedfsFilerPort)
	return seaweedfs_client.NewClient(
		&masterAddress,
		&filerAddress,
		nil,
	)
}

func NewSeaweedNodeInfo(logger *logrus.Entry, pod corev1.Pod) SeaweedfsNodeInfo {
	return SeaweedfsNodeInfo{
		logger: logger,
		Pod:    pod,
	}
}

type SeaweedfsInfo struct {
	logger    *logrus.Entry
	appLabels []string
}

func NewSeaweedfsInfo(logger *logrus.Entry) SeaweedfsInfo {
	return SeaweedfsInfo{
		logger:    logger,
		appLabels: pkg_cfg.SeaweedfsStaticPodLabelsSelector,
	}
}

func (s *SeaweedfsInfo) SeaweedfsInfoWaitByFunc(cmpFunc func(pods *corev1.PodList) bool) ([]SeaweedfsNodeInfo, error) {
	pods, isReady, err := kube_actions.WaitAppPodsInfo(s.appLabels, cmpFunc)
	if err != nil {
		return nil, err
	}
	if !isReady {
		return nil, fmt.Errorf("Service is not ready yet")
	}
	if pods == nil {
		return nil, nil
	}

	seaweedfsNodes := make([]SeaweedfsNodeInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		seaweedfsNodes = append(
			seaweedfsNodes,
			NewSeaweedNodeInfo(
				s.logger,
				pod,
			),
		)
	}
	return seaweedfsNodes, nil
}

func (s *SeaweedfsInfo) SeaweedfsInfoGet() ([]SeaweedfsNodeInfo, error) {
	pods, err := kube_actions.GetPodsInfoByLabels(s.appLabels)

	if err != nil {
		return nil, err
	}

	if pods == nil {
		return nil, nil
	}

	seaweedfsNodes := make([]SeaweedfsNodeInfo, 0, len(pods.Items))
	for _, pod := range pods.Items {
		seaweedfsNodes = append(
			seaweedfsNodes,
			NewSeaweedNodeInfo(
				s.logger,
				pod,
			),
		)
	}
	return seaweedfsNodes, nil
}
