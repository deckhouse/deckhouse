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
	"path"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/manager/packages"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	crdsDir = "crds"
)

type ClusterApplication struct {
	path string

	name    string
	version string

	dependencies map[string]string

	logger *log.Logger
}

func NewClusterApplication(path string, def *packages.Definition) *ClusterApplication {
	return &ClusterApplication{
		path: path,

		name:    def.Name,
		version: def.Version,

		dependencies: def.Requirements.Modules,

		logger: log.NewLogger().Named(def.Name),
	}
}

func (a *ClusterApplication) Name() string {
	return a.name
}

func (a *ClusterApplication) Version() string {
	return a.version
}

func (a *ClusterApplication) CRDs() string {
	return path.Join(a.path, crdsDir)
}
