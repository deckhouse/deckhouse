// Copyright 2025 Flant JSC
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

package statusmapper

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
)

// Input contains all data needed for condition evaluation.
// Create directly as a struct literal - no constructor needed.
type Input struct {
	// InternalConditions from the operator (keyed by condition name)
	InternalConditions map[status.ConditionName]status.Condition

	// ExternalConditions currently set on the resource (keyed by condition name)
	ExternalConditions map[status.ConditionName]status.Condition

	// VersionChanged indicates spec.version != status.currentVersion
	VersionChanged bool

	// IsInitialInstall indicates Installed condition was never True
	IsInitialInstall bool
}
