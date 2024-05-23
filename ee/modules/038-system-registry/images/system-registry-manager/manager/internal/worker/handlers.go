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
	http.HandleFunc(pkg_api.MasterInfoUrlPattern, pkg_api.CreateMasterInfoHandler(masterInfo))
	http.HandleFunc(pkg_api.CheckRegistryUrlPattern, pkg_api.CreateCheckRegistryHandler(checkRegistry))
	http.HandleFunc(pkg_api.UpdateRegistryUrlPattern, pkg_api.CreateUpdateRegistryHandler(updateRegistry))
	http.HandleFunc(pkg_api.DeleteRegistryUrlPattern, pkg_api.CreateDeleteRegistryHandler(deleteRegistry))
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

func checkRegistryHandlerFunc(_ *WorkerData, _ *pkg_api.CheckRegistryRequest) (*pkg_api.CheckRegistryResponse, error) {
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(manifestsSpec); err != nil {
		return nil, err
	}
	if err := steps.GenerateCerts(manifestsSpec); err != nil {
		return nil, err
	}
	if err := steps.CheckDestFiles(manifestsSpec); err != nil {
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
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.PrepareWorkspace(manifestsSpec); err != nil {
		return err
	}
	if err := steps.GenerateCerts(manifestsSpec); err != nil {
		return err
	}
	if err := steps.CheckDestFiles(manifestsSpec); err != nil {
		return err
	}
	if !manifestsSpec.NeedChange() {
		workerData.log.Info("No changes")
		return nil
	}
	if err := steps.UpdateManifests(manifestsSpec); err != nil {
		return err
	}
	return nil
}

func deleteRegistryHandlerFunc(_ *WorkerData) error {
	manifestsSpec := pkg_cfg.NewManifestsSpec()
	return steps.DeleteManifests(manifestsSpec)
}
