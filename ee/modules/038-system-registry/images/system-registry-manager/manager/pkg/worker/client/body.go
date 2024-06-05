/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

// CheckRegistry (Request + Response)
type CheckRegistryRequest struct {
	Seaweedfs struct {
		MasterPeers []string `json:"masterPeers"`
	} `json:"seaweedfs"`
}

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

// UpdateRegistry (Request + Response)
type UpdateRegistryRequest = CheckRegistryRequest

// MasterInfo (Request + Response)
type MasterInfoResponse struct {
	Data struct {
		IsMaster          bool   `json:"isMaster"`
		MasterName        string `json:"masterName"`
		CurrentMasterName string `json:"currentMasterName"`
	} `json:"data,omitempty"`
}

// Busy (Request + Response)
type IsBusyRequest struct {
	WaitTimeoutSeconds *int `json:"waitTimeoutSeconds"`
}

type IsBusyResponse struct {
	Data struct {
		IsBusy bool `json:"isBusy"`
	} `json:"data,omitempty"`
}
