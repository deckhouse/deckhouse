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

	createRegistry := func(requestBody *worker_client.CreateRegistryRequest) error {
		return createRegistryHandlerFunc(workerData, requestBody)
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
	http.HandleFunc(worker_client.CheckRegistryUrlPattern, worker_client.CreateCheckRegistryHandlerFunc(checkRegistry))
	http.Handle(worker_client.CreateRegistryUrlPattern, worker_client.CreateCreateRegistryHandler(createRegistry, workerData.singleRequestCfg))
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
			IsMaster          bool   `json:"isMaster"`
			MasterName        string `json:"masterName"`
			CurrentMasterName string `json:"currentMasterName"`
		}{
			IsMaster:          workerData.commonCfg.IsMaster(),
			MasterName:        workerData.commonCfg.MasterName(),
			CurrentMasterName: workerData.commonCfg.CurrentMasterName(),
		},
	}
	return &masterInfo, nil
}

func checkRegistryHandlerFunc(workerData *WorkerData, request *worker_client.CheckRegistryRequest) (*worker_client.CheckRegistryResponse, error) {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	params := steps.InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		StaticPods: struct {
			UpdateOrCreate       bool
			MasterPeers          []string
			CheckWithMasterPeers bool
		}{
			UpdateOrCreate:       true,
			CheckWithMasterPeers: request.CheckWithMasterPeers,
			MasterPeers:          request.MasterPeers,
		},
	}

	bundle, err := steps.CreateBundle(workerData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.CheckDest(workerData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return nil, err
	}
	return &worker_client.CheckRegistryResponse{
		Data: struct {
			RegistryFilesState struct {
				ManifestsIsExist         bool `json:"manifestsIsExist"`
				ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
				StaticPodsIsExist        bool `json:"staticPodsIsExist"`
				StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
				CertificateIsExist       bool `json:"certificateIsExist"`
				CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
			} `json:"registryState"`
		}{
			RegistryFilesState: struct {
				ManifestsIsExist         bool `json:"manifestsIsExist"`
				ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
				StaticPodsIsExist        bool `json:"staticPodsIsExist"`
				StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
				CertificateIsExist       bool `json:"certificateIsExist"`
				CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
			}{
				ManifestsIsExist:         bundle.ManifestsIsExist(),
				ManifestsWaitToUpdate:    bundle.ManifestsWaitToUpdate(),
				StaticPodsIsExist:        bundle.StaticPodsIsExist(),
				StaticPodsWaitToUpdate:   bundle.StaticPodsWaitToUpdate(),
				CertificateIsExist:       bundle.CertificateIsExist(),
				CertificatesWaitToUpdate: bundle.CertificatesWaitToUpdate(),
			},
		},
	}, nil
}

func updateRegistryHandlerFunc(workerData *WorkerData, request *worker_client.UpdateRegistryRequest) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	params := steps.InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: request.Certs.UpdateOrCreate},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: request.Manifests.UpdateOrCreate},
		StaticPods: struct {
			UpdateOrCreate       bool
			MasterPeers          []string
			CheckWithMasterPeers bool
		}{
			UpdateOrCreate:       request.StaticPods.UpdateOrCreate,
			CheckWithMasterPeers: true,
			MasterPeers:          request.StaticPods.MasterPeers,
		},
	}

	bundle, err := steps.CreateBundle(workerData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDest(workerData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.Update(workerData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.PatchStaticPodsDestForRestart(workerData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func createRegistryHandlerFunc(workerData *WorkerData, request *worker_client.CreateRegistryRequest) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	params := steps.InputParams{
		Certs:     struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		Manifests: struct{ UpdateOrCreate bool }{UpdateOrCreate: true},
		StaticPods: struct {
			UpdateOrCreate       bool
			MasterPeers          []string
			CheckWithMasterPeers bool
		}{
			UpdateOrCreate:       true,
			CheckWithMasterPeers: true,
			MasterPeers:          request.MasterPeers,
		},
	}

	bundle, err := steps.CreateBundle(workerData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDest(workerData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.Update(workerData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.PatchStaticPodsDestForRestart(workerData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func deleteRegistryHandlerFunc(workerData *WorkerData) error {
	log := workerData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.Delete(workerData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
