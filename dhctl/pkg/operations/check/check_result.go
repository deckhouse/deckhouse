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

package check

import "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"

type CheckStatus string

const (
	CheckStatusInSync               CheckStatus = "InSync"
	CheckStatusOutOfSync            CheckStatus = "OutOfSync"
	CheckStatusDestructiveOutOfSync CheckStatus = "DestructiveOutOfSync"
)

func (status CheckStatus) CombineStatus(anotherStatus CheckStatus) CheckStatus {
	// NOTE: logic is to downgrade status if another status is "worse" than current
	switch status {
	case CheckStatusInSync:
		if anotherStatus != CheckStatusInSync {
			return anotherStatus
		}
	case CheckStatusOutOfSync:
		if anotherStatus == CheckStatusDestructiveOutOfSync {
			return CheckStatusDestructiveOutOfSync
		}
	case CheckStatusDestructiveOutOfSync:
	}
	return status
}

type StatusDetails struct {
	ConfigurationStatus CheckStatus `json:"configuration_status"`
	converge.Statistics `json:",inline"`
}

type CheckResult struct {
	Status              CheckStatus   `json:"status"`
	StatusDetails       StatusDetails `json:"status_details"`
	DestructiveChangeID string        `json:"destructive_change_id,omitempty"`
}
