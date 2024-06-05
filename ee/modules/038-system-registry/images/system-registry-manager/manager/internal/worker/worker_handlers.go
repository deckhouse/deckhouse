/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package worker

import (
	"fmt"
	"net/http"
	"system-registry-manager/internal/worker/steps"
	pkg_cfg "system-registry-manager/pkg/cfg"
	worker_client "system-registry-manager/pkg/worker/client"
)

func createServer(workerData *WorkerData) *http.Server {
	server := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", (*pkg_cfg.GetConfig()).Manager.WorkerPort),
	}

	masterInfo := func() (*worker_client.MasterInfoResponse, error) {
		return masterInfoHandlerFunc(workerData)
	}

	checkRegistry := func(requestBody *worker_client.CheckRegistryRequest) (*worker_client.CheckRegistryResponse, error) {
		return checkRegistryHandlerFunc(workerData, requestBody)
	}

	updateRegistry := func(requestBody *worker_client.UpdateRegistryRequest) error {
		return updateRegistryHandlerFunc(workerData, requestBody)
	}

	deleteRegistry := func() error {
		return deleteRegistryHandlerFunc(workerData)
	}

	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)
	http.HandleFunc(worker_client.MasterInfoUrlPattern, worker_client.CreateMasterInfoHandlerFunc(masterInfo))
	http.HandleFunc(worker_client.IsBusyUrlPattern, worker_client.CreateIsBusyHandlerFunc(workerData.singleRequestCfg))
	http.Handle(worker_client.CheckRegistryUrlPattern, worker_client.CreateCheckRegistryHandler(checkRegistry, workerData.singleRequestCfg))
	http.Handle(worker_client.UpdateRegistryUrlPattern, worker_client.CreateUpdateRegistryHandler(updateRegistry, workerData.singleRequestCfg))
	http.Handle(worker_client.DeleteRegistryUrlPattern, worker_client.CreateDeleteRegistryHandler(deleteRegistry, workerData.singleRequestCfg))
	return server
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func masterInfoHandlerFunc(workerData *WorkerData) (*worker_client.MasterInfoResponse, error) {
	masterInfo := worker_client.MasterInfoResponse{
		Data: struct {
			IsMaster          bool   "json:\"isMaster\""
			MasterName        string "json:\"masterName\""
			CurrentMasterName string "json:\"currentMasterName\""
		}{
			IsMaster:          workerData.commonCfg.IsMaster(),
			MasterName:        workerData.commonCfg.MasterName(),
			CurrentMasterName: workerData.commonCfg.CurrentMasterName(),
		},
	}
	return &masterInfo, nil
}

func checkRegistryHandlerFunc(workerData *WorkerData, _ *worker_client.CheckRegistryRequest) (*worker_client.CheckRegistryResponse, error) {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.GenerateCerts(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.CheckDestFiles(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if !manifestsSpec.NeedChange() {
		return &worker_client.CheckRegistryResponse{}, nil
	}
	return &worker_client.CheckRegistryResponse{
		Data: struct {
			RegistryFilesState struct {
				ManifestsWaitToCreate    bool "json:\"manifestsWaitToCreate\""
				ManifestsWaitToUpdate    bool "json:\"manifestsWaitToUpdate\""
				StaticPodsWaitToCreate   bool "json:\"staticPodsWaitToCreate\""
				StaticPodsWaitToUpdate   bool "json:\"staticPodsWaitToUpdate\""
				CertificatesWaitToCreate bool "json:\"certificatesWaitToCreate\""
				CertificatesWaitToUpdate bool "json:\"certificatesWaitToUpdate\""
			} "json:\"registryState\""
		}{
			RegistryFilesState: struct {
				ManifestsWaitToCreate    bool "json:\"manifestsWaitToCreate\""
				ManifestsWaitToUpdate    bool "json:\"manifestsWaitToUpdate\""
				StaticPodsWaitToCreate   bool "json:\"staticPodsWaitToCreate\""
				StaticPodsWaitToUpdate   bool "json:\"staticPodsWaitToUpdate\""
				CertificatesWaitToCreate bool "json:\"certificatesWaitToCreate\""
				CertificatesWaitToUpdate bool "json:\"certificatesWaitToUpdate\""
			}{
				ManifestsWaitToCreate:    manifestsSpec.NeedManifestsCreate(),
				ManifestsWaitToUpdate:    manifestsSpec.NeedManifestsUpdate(),
				StaticPodsWaitToCreate:   manifestsSpec.NeedStaticPodsCreate(),
				StaticPodsWaitToUpdate:   manifestsSpec.NeedStaticPodsUpdate(),
				CertificatesWaitToCreate: manifestsSpec.NeedStaticCertificatesCreate(),
				CertificatesWaitToUpdate: manifestsSpec.NeedStaticCertificatesUpdate(),
			},
		},
	}, nil
}

func updateRegistryHandlerFunc(workerData *WorkerData, _ *worker_client.CheckRegistryRequest) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.GenerateCerts(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDestFiles(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if !manifestsSpec.NeedChange() {
		log.Debug("No changes")
		return nil
	}
	if err := steps.UpdateManifests(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func deleteRegistryHandlerFunc(workerData *WorkerData) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.DeleteManifests(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
