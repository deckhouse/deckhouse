/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"context"
	"system-registry-manager/internal/master/worker"
	pkg_api "system-registry-manager/pkg/api"
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
	workersInfo := worker.NewWorkersInfo()
	workers, err := workersInfo.WaitWorkers()
	if err != nil {
		return err
	}
	for _, worker := range workers {
		resp, err := worker.Client.RequestCheckRegistry(&pkg_api.CheckRegistryRequest{})
		if err != nil {
			return err
		}
		if !(resp.Data.RegistryFilesState.ManifestsWaitToCreate ||
			resp.Data.RegistryFilesState.ManifestsWaitToUpdate ||
			resp.Data.RegistryFilesState.StaticPodsWaitToCreate ||
			resp.Data.RegistryFilesState.StaticPodsWaitToUpdate ||
			resp.Data.RegistryFilesState.CertificatesWaitToCreate ||
			resp.Data.RegistryFilesState.CertificatesWaitToUpdate) {
			return nil
		}
		err = worker.Client.RequestUpdateRegistry(&pkg_api.CheckRegistryRequest{})
		if err != nil {
			return err
		}
	}
	return nil
}
