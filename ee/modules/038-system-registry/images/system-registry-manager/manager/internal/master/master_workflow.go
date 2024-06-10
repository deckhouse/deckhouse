/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"context"
	k8s_info "system-registry-manager/internal/master/k8s_info"
	"system-registry-manager/internal/master/workflow"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_logs "system-registry-manager/pkg/logs"
	"time"
)

const (
	workInterval = 10 * time.Second
)

func startMasterWorkflow(ctx context.Context, m *Master) {
	log := pkg_logs.GetLoggerFromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(workInterval)
			err := masterWorkflow(ctx, m)
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
}

func masterWorkflow(ctx context.Context, m *Master) error {
	workersInfo, err := k8s_info.WaitAllWorkers()
	if err != nil {
		return err
	}

	nodeManagers := make([]workflow.NodeManager, 0, len(workersInfo))

	for _, workerInfo := range workersInfo {
		nodeManagers = append(nodeManagers, NewNodeManager(ctx, workerInfo))
	}

	seaweedfsCaCertsWorkflow := workflow.NewSeaweedfsCertsWorkflow(ctx, nodeManagers, pkg_cfg.GetConfig().Cluster.Size)
	err = seaweedfsCaCertsWorkflow.Start()
	if err != nil {
		return err
	}

	seaweedfsScaleWorkflow := workflow.NewSeaweedfsScaleWorkflow(ctx, nodeManagers, pkg_cfg.GetConfig().Cluster.Size)
	err = seaweedfsScaleWorkflow.Start()
	if err != nil {
		return err
	}
	return nil
}
