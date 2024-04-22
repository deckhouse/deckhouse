/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8sinfo

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_k8s_act "system-registry-manager/pkg/kubernetes/actions"
)

func GetSeaweedfsPodByNodeName(nodeNmae string) (*corev1.Pod, error) {
	cfg := pkg_cfg.GetConfig()
	seaweedfsPods, err := pkg_k8s_act.GetPodsInfoByLabels(cfg.Manager.Namespace, pkg_cfg.SeaweedfsStaticPodLabelsSelector)
	if err != nil {
		return nil, err
	}
	for _, pod := range seaweedfsPods.Items {
		if pod.Spec.NodeName == nodeNmae {
			return &pod, nil
		}
	}
	return nil, nil
}

func WaitExecutorDaemonset() (*appsv1.DaemonSet, error) {
	cfg := pkg_cfg.GetConfig()
	daemonset, isWaited, err := pkg_k8s_act.WaitDaemonsetInfo(cfg.Manager.Namespace, cfg.Manager.DaemonsetName, pkg_k8s_act.DaemonsetCmpFuncEqualDesiredAndReady)
	if err != nil {
		return nil, err
	}
	if !isWaited {
		return nil, fmt.Errorf("error WaitDaemonsetPods")
	}
	return daemonset, nil
}

func WaitExecutorEndpoints() (*corev1.Endpoints, error) {
	cfg := pkg_cfg.GetConfig()
	ep, isWaited, err := pkg_k8s_act.WaitEndpointInfo(cfg.Manager.Namespace, cfg.Manager.ServiceName, pkg_k8s_act.EndpointCmpNotReadyAddressesEmpty)
	if err != nil {
		return nil, err
	}
	if !isWaited {
		return nil, fmt.Errorf("error WaitExecutorEndpoints")
	}
	return ep, nil
}

func WaitAllExecutors() ([]ExecutorInfo, error) {
	dsInfo, err := WaitExecutorDaemonset()
	if err != nil {
		return nil, err
	}

	epInfo, err := WaitExecutorEndpoints()
	if err != nil {
		return nil, err
	}

	numberOfNode := dsInfo.Status.DesiredNumberScheduled

	if len(epInfo.Subsets) == 0 {
		return nil, fmt.Errorf("error len(ep.Subsets) == 0")
	}
	if len(epInfo.Subsets[0].Addresses) != int(numberOfNode) {
		return nil, fmt.Errorf("error len(ep.Subsets[0].Addresses) != numberOfNode")
	}

	masterNodes, err := pkg_k8s_act.GetMasterNodesInfo()
	mergedInfos := make([]ExecutorInfo, 0, len(masterNodes.Items))
	for _, masterNode := range masterNodes.Items {
		mergedInfo, err := executorInfoByNode(&masterNode, epInfo)
		if err != nil {
			return nil, err
		}
		mergedInfos = append(mergedInfos, *mergedInfo)
	}
	return mergedInfos, nil
}

func executorInfoByNode(nodeInfo *corev1.Node, epsInfo *corev1.Endpoints) (*ExecutorInfo, error) {
	if nodeInfo == nil {
		return nil, fmt.Errorf("node == nil")
	}
	epInfo := getExecutorEndpointByNodeName(nodeInfo.Name, epsInfo)
	if epInfo == nil {
		return nil, fmt.Errorf("ep == nil")
	}
	return &ExecutorInfo{
		MasterNode: *nodeInfo,
		Executor:   *epInfo,
	}, nil
}

func getExecutorEndpointByNodeName(nodeNmae string, ep *corev1.Endpoints) *corev1.EndpointAddress {
	if ep == nil {
		return nil
	}
	for _, subsets := range ep.Subsets {
		for _, address := range subsets.Addresses {
			if address.NodeName != nil && *address.NodeName == nodeNmae {
				return &address
			}
		}
	}
	return nil
}

type ExecutorInfo struct {
	MasterNode corev1.Node
	Executor   corev1.EndpointAddress
}
