/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package worker

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	pkg_api "system-registry-manager/pkg/api"
	pkg_cfg "system-registry-manager/pkg/cfg"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
)

type WorkersInfo struct {
	nsName string
	dsName string
	svName string
}

type WorkerInfo struct {
	UID      string
	Ip       string
	PodName  string
	NodeName *string
	Client   *pkg_api.Client
}

func NewWorkersInfo() WorkersInfo {
	cfg := pkg_cfg.GetConfig()
	return WorkersInfo{
		nsName: cfg.Manager.Namespace,
		dsName: cfg.Manager.DaemonsetName,
		svName: cfg.Manager.ServiceName,
	}
}

func NewWorkerInfo(uID, ip, podName string, nodeName *string) WorkerInfo {
	return WorkerInfo{
		UID:      uID,
		Ip:       ip,
		PodName:  podName,
		NodeName: nodeName,
		Client:   pkg_api.NewClient(ip, pkg_cfg.GetConfig().Manager.WorkerPort),
	}
}

func (w *WorkersInfo) WaitWorkers() ([]WorkerInfo, error) {
	var numberOfNode int
	var endpoints corev1.Endpoints
	{
		ds, isReady, err := kube_actions.WaitDaemonsetInfo(w.nsName, w.dsName, kube_actions.DaemonsetCmpFuncEqualDesiredAndReady)
		if err != nil {
			return nil, err
		}
		if !isReady {
			return nil, fmt.Errorf("Daemonset is not ready yet")
		}
		numberOfNode = int(ds.Status.DesiredNumberScheduled)
	}

	{
		ep, isReady, err := kube_actions.WaitEndpointInfo(w.nsName, w.svName, kube_actions.EndpointCmpNotReadyAddressesEmpty)
		if err != nil {
			return nil, err
		}
		if !isReady {
			return nil, fmt.Errorf("Service is not ready yet")
		}
		endpoints = *ep
	}

	if len(endpoints.Subsets) == 0 {
		return nil, fmt.Errorf("Error len(ep.Subsets) == 0")
	}
	if len(endpoints.Subsets[0].Addresses) != numberOfNode {
		return nil, fmt.Errorf("Error len(ep.Subsets[0].Addresses) != numberOfNode")
	}

	workers := make([]WorkerInfo, 0, len(endpoints.Subsets[0].Addresses))
	for _, address := range endpoints.Subsets[0].Addresses {
		workers = append(
			workers,
			NewWorkerInfo(
				string(address.TargetRef.UID),
				address.IP,
				address.TargetRef.Name,
				address.NodeName,
			),
		)
	}
	return workers, nil
}
