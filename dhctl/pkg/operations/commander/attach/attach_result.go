// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package attach

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/check"
)

type AttachStatus string

const (
	StatusScanned  AttachStatus = "Scanned"
	StatusAttached AttachStatus = "Attached"
)

type ScanResult struct {
	ClusterConfiguration                 string `json:"cluster_configuration"`
	ProviderSpecificClusterConfiguration string `json:"provider_specific_cluster_configuration"`
	SSHPrivateKey                        string `json:"ssh_private_key"`
	SSHPublicKey                         string `json:"ssh_public_key"`
}

type AttachResult struct {
	Status      AttachStatus       `json:"status"`
	ScanResult  *ScanResult        `json:"scan_result"`
	CheckResult *check.CheckResult `json:"check_result,omitempty"`
}
