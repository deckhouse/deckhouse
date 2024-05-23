/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

// type CommonError = string

// CheckRegistryRequest represents the request structure for checking the registry.
type CheckRegistryRequest struct {
	Seaweedfs struct {
		MasterPeers []string `json:"masterPeers"`
	} `json:"seaweedfs"`
}

type UpdateRegistryRequest = CheckRegistryRequest

// CheckRegistryResponse represents the response structure for checking the registry.
type CheckRegistryResponse struct {
	Data struct {
		RegistryFilesState struct {
			ManifestsWaitToCreate    bool `json:"manifestsWaitToCreate"`
			ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
			StaticPodsWaitToCreate   bool `json:"staticPodsWaitToCreate"`
			StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
			CertificatesWaitToCreate bool `json:"certificatesWaitToCreate"`
			CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
		} `json:"registryState"`
	} `json:"data,omitempty"`
}

// MasterInfoResponse represents the response structure for master info.
type MasterInfoResponse struct {
	Data struct {
		IsMaster          bool   `json:"isMaster"`
		MasterName        string `json:"masterName"`
		CurrentMasterName string `json:"currentMasterName"`
	} `json:"data,omitempty"`
}

type BusyResponce struct {
	Data struct {
		IsBusy bool `json:"isBusy"`
	} `json:"data,omitempty"`
}
