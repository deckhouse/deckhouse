/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package worker

import (
	"net/http"
	"system-registry-manager/internal/worker/steps"
	pkg_api "system-registry-manager/pkg/api"
	pkg_cfg "system-registry-manager/pkg/cfg"
)

func createServer(workerData *WorkerData) *http.Server {
	server := &http.Server{
		Addr: serverAddr,
	}

	masterInfo := func() (*pkg_api.MasterInfoResponse, error) {
		return masterInfoHandlerFunc(workerData)
	}

	checkRegistry := func(requestBody *pkg_api.CheckRegistryRequest) (*pkg_api.CheckRegistryResponse, error) {
		return checkRegistryHandlerFunc(workerData, requestBody)
	}

	updateRegistry := func(requestBody *pkg_api.UpdateRegistryRequest) error {
		return updateRegistryHandlerFunc(workerData, requestBody)
	}

	deleteRegistry := func() error {
		return deleteRegistryHandlerFunc(workerData)
	}

	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)
	http.HandleFunc(pkg_api.MasterInfoUrlPattern, pkg_api.CreateMasterInfoHandlerFunc(masterInfo))
	http.HandleFunc(pkg_api.IsBusyUrlPattern, pkg_api.CreateIsBusyHandlerFunc(workerData.singleRequestCfg))
	http.Handle(pkg_api.CheckRegistryUrlPattern, pkg_api.CreateCheckRegistryHandler(checkRegistry, workerData.singleRequestCfg))
	http.Handle(pkg_api.UpdateRegistryUrlPattern, pkg_api.CreateUpdateRegistryHandler(updateRegistry, workerData.singleRequestCfg))
	http.Handle(pkg_api.DeleteRegistryUrlPattern, pkg_api.CreateDeleteRegistryHandler(deleteRegistry, workerData.singleRequestCfg))
	return server
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func masterInfoHandlerFunc(workerData *WorkerData) (*pkg_api.MasterInfoResponse, error) {
	masterInfo := pkg_api.MasterInfoResponse{
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

func checkRegistryHandlerFunc(workerData *WorkerData, _ *pkg_api.CheckRegistryRequest) (*pkg_api.CheckRegistryResponse, error) {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.GenerateCerts(manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.CheckDestFiles(manifestsSpec); err != nil {
		log.Error(err)
		return nil, err
	}
	if !manifestsSpec.NeedChange() {
		return &pkg_api.CheckRegistryResponse{}, nil
	}
	return &pkg_api.CheckRegistryResponse{
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

func updateRegistryHandlerFunc(workerData *WorkerData, _ *pkg_api.CheckRegistryRequest) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.GenerateCerts(manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDestFiles(manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	if !manifestsSpec.NeedChange() {
		log.Info("No changes")
		return nil
	}
	if err := steps.UpdateManifests(manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func deleteRegistryHandlerFunc(workerData *WorkerData) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.DeleteManifests(manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
