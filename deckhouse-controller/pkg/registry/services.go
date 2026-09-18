// Copyright 2026 Flant JSC
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

package registry

import (
	"errors"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Errors the commands report to the user. The registry itself answers
// "not found" for both cases, so they exist only to phrase it.
var (
	ErrChannelIsNotFound = errors.New("channel is not found")
	ErrModuleIsNotFound  = errors.New("module is not found")
)

// deckhouseRegistry opens the Deckhouse tree at the address the registry secret
// holds. That address already carries the edition, so it is the edition root
// and the release channel hangs off it.
func deckhouseRegistry(repo string, config *utils.RegistryConfig, logger *log.Logger) (*dhregistry.Registry, error) {
	cli, err := newRegistryClient(repo, config, logger)
	if err != nil {
		return nil, err
	}

	return dhregistry.NewForPath(cli, dhregistry.WithLogger(logger)), nil
}

// moduleCatalog opens the module catalog a ModuleSource points at. Its
// spec.registry.repo is the catalog itself — its tags are module names — so the
// repository is wrapped as a catalog directly, with no path surgery.
func moduleCatalog(repo string, config *utils.RegistryConfig, logger *log.Logger) (*module.Catalog, error) {
	cli, err := newRegistryClient(repo, config, logger)
	if err != nil {
		return nil, err
	}

	return module.NewCatalog(service.NewBasicService(module.CatalogServiceName, cli, logger)), nil
}
