/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
)

const (
	moduleSegment   = "modules"
	packagesSegment = "packages"
)

// Service provides high-level registry operations using a registry client
type Service struct {
	client registry.Client

	// modulesService   *BasicService
	packagesManager  *PackageServiceManager
	deckhouseService *DeckhouseService

	logger *log.Logger
}

// NewService creates a new registry service with the given client and logger
func NewService(client registry.Client, logger *log.Logger) *Service {
	s := &Service{
		client: client,
		logger: logger,
	}

	// s.modulesService = NewModulesService(client.WithSegment(moduleSegment), logger.Named("modules"))
	s.packagesManager = NewPackageServiceManager(logger.Named("packages_manager"))
	s.deckhouseService = NewDeckhouseService(client, logger.Named("deckhouse"))

	return s
}

// GetRoot gets path of the registry root
func (s *Service) GetRoot() string {
	return s.client.GetRegistry()
}

// ModuleService returns the module service
// func (s *Service) ModuleService() *ModulesService {
// 	return s.modulesService
// }

// PackagesService returns the packages service
func (s *Service) PackagesService(registryURL string, dockerCFG string, ca string, userAgent string, scheme string) (*PackagesService, error) {
	return s.packagesManager.PackagesService(registryURL, dockerCFG, ca, userAgent, scheme)
}

// DeckhouseService returns the deckhouse service
func (s *Service) DeckhouseService() *DeckhouseService {
	return s.deckhouseService
}
