/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

// CheckRegistry (Request + Response)
type CheckRegistryRequest struct {
	Options struct {
		MasterPeers     []string `json:"masterPeers"`
		IsRaftBootstrap bool     `json:"isRaftBootstrap"`
	} `json:"options"`
	Check struct {
		WithMasterPeers     bool `json:"withMasterPeers"`
		WithIsRaftBootstrap bool `json:"withIsRaftBootstrap"`
	} `json:"check"`
}

type UpdateRegistryRequest struct {
	StaticPods struct {
		MasterPeers     []string `json:"masterPeers"`
		IsRaftBootstrap bool     `json:"isRaftBootstrap"`
		UpdateOrCreate  bool     `json:"updateOrCreate"`
	} `json:"staticPods"`
	Certs struct {
		UpdateOrCreate bool `json:"updateOrCreate"`
	} `json:"certs"`
	Manifests struct {
		UpdateOrCreate bool `json:"updateOrCreate"`
	} `json:"manifests"`
}

type CreateRegistryRequest struct {
	MasterPeers     []string `json:"masterPeers"`
	IsRaftBootstrap bool     `json:"isRaftBootstrap"`
}

// Busy (Request + Response)
type IsBusyRequest struct {
	WaitTimeoutSeconds *int `json:"waitTimeoutSeconds"`
}

type CheckRegistryResponse struct {
	Data struct {
		RegistryFilesState struct {
			ManifestsIsExist         bool `json:"manifestsIsExist"`
			ManifestsWaitToUpdate    bool `json:"manifestsWaitToUpdate"`
			StaticPodsIsExist        bool `json:"staticPodsIsExist"`
			StaticPodsWaitToUpdate   bool `json:"staticPodsWaitToUpdate"`
			CertificateIsExist       bool `json:"certificateIsExist"`
			CertificatesWaitToUpdate bool `json:"certificatesWaitToUpdate"`
		} `json:"registryState"`
	} `json:"data,omitempty"`
}

func (resp *CheckRegistryResponse) NeedSomethingCreateOrUpdate() bool {
	return (resp.Data.RegistryFilesState.StaticPodsWaitToUpdate || !resp.Data.RegistryFilesState.StaticPodsIsExist) ||
		(resp.Data.RegistryFilesState.ManifestsWaitToUpdate || !resp.Data.RegistryFilesState.ManifestsIsExist) ||
		(resp.Data.RegistryFilesState.CertificatesWaitToUpdate || !resp.Data.RegistryFilesState.CertificateIsExist)
}

// MasterInfo (Request + Response)
type MasterInfoResponse struct {
	Data struct {
		IsMaster          bool   `json:"isMaster"`
		MasterName        string `json:"masterName"`
		CurrentMasterName string `json:"currentMasterName"`
	} `json:"data,omitempty"`
}

type IsBusyResponse struct {
	Data struct {
		IsBusy bool `json:"isBusy"`
	} `json:"data,omitempty"`
}
