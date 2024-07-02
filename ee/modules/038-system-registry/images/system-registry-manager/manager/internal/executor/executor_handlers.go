/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package executor

import (
	"fmt"
	"net/http"
	"system-registry-manager/internal/executor/steps"
	pkg_cfg "system-registry-manager/pkg/cfg"
	executor_client "system-registry-manager/pkg/executor/client"
)

func createServer(executorData *ExecutorData) *http.Server {
	server := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", (*pkg_cfg.GetConfig()).Manager.ExecutorPort),
	}

	masterInfo := func() (*executor_client.MasterInfoResponse, error) {
		return masterInfoHandlerFunc(executorData)
	}

	checkRegistry := func(requestBody *executor_client.CheckRegistryRequest) (*executor_client.CheckRegistryResponse, error) {
		return checkRegistryHandlerFunc(executorData, requestBody)
	}

	createRegistry := func(requestBody *executor_client.CreateRegistryRequest) error {
		return createRegistryHandlerFunc(executorData, requestBody)
	}

	updateRegistry := func(requestBody *executor_client.UpdateRegistryRequest) error {
		return updateRegistryHandlerFunc(executorData, requestBody)
	}

	deleteRegistry := func() error {
		return deleteRegistryHandlerFunc(executorData)
	}

	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)
	http.HandleFunc(executor_client.MasterInfoUrlPattern, executor_client.CreateMasterInfoHandlerFunc(masterInfo))
	http.HandleFunc(executor_client.IsBusyUrlPattern, executor_client.CreateIsBusyHandlerFunc(executorData.singleRequestCfg))
	http.HandleFunc(executor_client.CheckRegistryUrlPattern, executor_client.CreateCheckRegistryHandlerFunc(checkRegistry))
	http.Handle(executor_client.CreateRegistryUrlPattern, executor_client.CreateCreateRegistryHandler(createRegistry, executorData.singleRequestCfg))
	http.Handle(executor_client.UpdateRegistryUrlPattern, executor_client.CreateUpdateRegistryHandler(updateRegistry, executorData.singleRequestCfg))
	http.Handle(executor_client.DeleteRegistryUrlPattern, executor_client.CreateDeleteRegistryHandler(deleteRegistry, executorData.singleRequestCfg))
	return server
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func masterInfoHandlerFunc(executorData *ExecutorData) (*executor_client.MasterInfoResponse, error) {
	masterInfo := executor_client.MasterInfoResponse{
		Data: struct {
			IsMaster          bool   `json:"isMaster"`
			MasterName        string `json:"masterName"`
			CurrentMasterName string `json:"currentMasterName"`
		}{
			IsMaster:          executorData.commonCfg.IsMaster(),
			MasterName:        executorData.commonCfg.MasterName(),
			CurrentMasterName: executorData.commonCfg.CurrentMasterName(),
		},
	}
	return &masterInfo, nil
}

func checkRegistryHandlerFunc(executorData *ExecutorData, request *executor_client.CheckRegistryRequest) (*executor_client.CheckRegistryResponse, error) {
	log := executorData.log
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

	bundle, err := steps.CreateBundle(executorData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if err := steps.CheckDest(executorData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return nil, err
	}
	return &executor_client.CheckRegistryResponse{
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

func updateRegistryHandlerFunc(executorData *ExecutorData, request *executor_client.UpdateRegistryRequest) error {
	log := executorData.log
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

	bundle, err := steps.CreateBundle(executorData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDest(executorData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.Update(executorData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.PatchStaticPodsDestForRestart(executorData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func createRegistryHandlerFunc(executorData *ExecutorData, request *executor_client.CreateRegistryRequest) error {
	log := executorData.log
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

	bundle, err := steps.CreateBundle(executorData.rootCtx, manifestsSpec, &params)
	if err != nil {
		log.Error(err)
		return err
	}
	if err := steps.CheckDest(executorData.rootCtx, bundle, &params); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.Update(executorData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	if err := steps.PatchStaticPodsDestForRestart(executorData.rootCtx, bundle); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func deleteRegistryHandlerFunc(executorData *ExecutorData) error {
	log := executorData.log
	manifestsSpec := pkg_cfg.NewManifestsSpec()

	if err := steps.Delete(executorData.rootCtx, manifestsSpec); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
