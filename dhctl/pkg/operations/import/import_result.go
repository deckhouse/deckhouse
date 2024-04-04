package _import

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
)

type ImportStatus string

const (
	ImportStatusScanned  ImportStatus = "Scanned"
	ImportStatusImported ImportStatus = "Imported"
)

type ScanResult struct {
	ClusterConfiguration                 string `json:"cluster_configuration"`
	ProviderSpecificClusterConfiguration string `json:"provider_specific_cluster_configuration"`
}

type ImportResult struct {
	Status      ImportStatus       `json:"status"`
	ScanResult  *ScanResult        `json:"scan_result"`
	CheckResult *check.CheckResult `json:"check_result,omitempty"`
}
