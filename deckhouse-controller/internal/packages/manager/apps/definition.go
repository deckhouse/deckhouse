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

package apps

import (
	"github.com/Masterminds/semver/v3"
)

// Definition represents application metadata.
type Definition struct {
	Name    string
	Version string
	Stage   string

	Requirements   Requirements
	DisableOptions DisableOptions
}

// Requirements specifies dependencies required by the application.
type Requirements struct {
	Kubernetes *semver.Constraints
	Deckhouse  *semver.Constraints
	Modules    map[string]*semver.Constraints
}

// DisableOptions configures application disablement behavior.
type DisableOptions struct {
	Confirmation bool   // Whether confirmation is required to disable
	Message      string // Message to display when disabling
}
