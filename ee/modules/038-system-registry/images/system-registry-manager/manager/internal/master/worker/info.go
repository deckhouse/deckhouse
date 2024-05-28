/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package worker

import (
	"fmt"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	pkg_api "system-registry-manager/pkg/api"
	pkg_cfg "system-registry-manager/pkg/cfg"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"
)

type WorkersInfo struct {
	logger *logrus.Entry
	nsName string
	dsName string
	svName string
}

type WorkerInfo struct {
	logger   *logrus.Entry
	UID      string
	Ip       string
	PodName  string
	NodeName *string
	Client   *pkg_api.Client
}

func NewWorkersInfo(logger *logrus.Entry) WorkersInfo {
	cfg := pkg_cfg.GetConfig()
	return WorkersInfo{
		logger: logger,
		nsName: cfg.Manager.Namespace,
		dsName: cfg.Manager.DaemonsetName,
		svName: cfg.Manager.ServiceName,
	}
}

func NewWorkerInfo(logger *logrus.Entry, uID, ip, podName string, nodeName *string) WorkerInfo {
	return WorkerInfo{
		logger:   logger,
		UID:      uID,
		Ip:       ip,
		PodName:  podName,
		NodeName: nodeName,
		Client:   pkg_api.NewClient(logger, ip, pkg_cfg.GetConfig().Manager.WorkerPort),
	}
}

func (w *WorkersInfo) WaitWorkers() ([]WorkerInfo, error) {
	w.logger.Debugf("Waiting for workers information...")

	var numberOfNode int
	var endpoints corev1.Endpoints

	// Wait for Daemonset information
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

	// Wait for Service Endpoint information
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
				w.logger,
				string(address.TargetRef.UID),
				address.IP,
				address.TargetRef.Name,
				address.NodeName,
			),
		)
		w.logger.Tracef("Found '%s' worker with address '%s'", address.TargetRef.Name, address.IP)
	}
	return workers, nil
}
