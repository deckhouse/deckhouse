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
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Application struct {
	path string

	name      string
	namespace string
	version   string

	dependencies map[string]string

	logger *log.Logger
}

func NewApplication(path, name, namespace, version string, deps map[string]string) *Application {
	return &Application{
		path: path,

		name:      name,
		namespace: namespace,
		version:   version,

		dependencies: deps,

		logger: log.NewLogger().Named(name),
	}
}

func (a *Application) Name() string {
	return a.name
}

func (a *Application) Namespace() string {
	return a.namespace
}

func (a *Application) Version() string {
	return a.version
}
