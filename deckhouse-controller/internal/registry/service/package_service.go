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
	"context"
	"errors"
	"fmt"
	"reflect"

	registryClient "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/client"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/packages"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/registry/client"
)

const (
	packagesServiceName       = "packages"
	packageServiceName        = "package"
	packageVersionServiceName = "package_version"
	packageReleaseServiceName = "package_release"
)

type ServiceManagerInterface[T any] interface {
	Service(registryURL string, config utils.RegistryConfig) (*T, error)
}

type ServiceManager[T any] struct {
	cachedCredentials map[string]*packageCredentials

	services map[packageCredentials]*T

	logger *log.Logger
}

type packageCredentials struct {
	registryURL string
	dockerCFG   string
	login       string
	password    string
	ca          string
	userAgent   string
}

func NewPackageServiceManager(logger *log.Logger) *ServiceManager[PackagesService] {
	return &ServiceManager[PackagesService]{
		cachedCredentials: make(map[string]*packageCredentials),
		services:          make(map[packageCredentials]*PackagesService),
		logger:            logger,
	}
}

func (m *ServiceManager[T]) Service(registryURL string, config utils.RegistryConfig) (*T, error) {
	if m.services == nil {
		m.services = make(map[packageCredentials]*T)
	}

	// Check for service injected via SetPackagesService (testing) with only registryURL
	testCreds := packageCredentials{
		registryURL: registryURL,
	}
	if svc, exists := m.services[testCreds]; exists {
		return svc, nil
	}

	creds := packageCredentials{
		registryURL: registryURL,
		dockerCFG:   config.DockerConfig,
		login:       config.Login,
		password:    config.Password,
		ca:          config.CA,
		userAgent:   config.UserAgent,
	}

	// if service with these creds already exists - return it
	_, svcExists := m.services[creds]
	if svcExists {
		return m.services[creds], nil
	}

	authOpts, err := m.createAuthOptions(registryURL, config.DockerConfig, config.Login, config.Password) // factory method
	if err != nil {
		return nil, fmt.Errorf("failed to get auth from docker config: %w", err)
	}

	// remove cached service with old credentials for this registryURL
	cachedCreds, isCached := m.cachedCredentials[registryURL]
	if isCached {
		delete(m.services, *cachedCreds)
		m.cachedCredentials[registryURL] = &creds
	}

	c := registryClient.New(registryURL,
		append(authOpts,
			client.WithInsecure(config.Scheme == "http"),
			client.WithCA(config.CA),
			client.WithUserAgent(config.UserAgent),
			client.WithLogger(m.logger),
		)...,
	)

	var zero T
	switch any(zero).(type) {
	case PackagesService, *PackagesService:
		m.services[creds] = any(NewPackagesService(c, m.logger)).(*T)
	default:
		return nil, fmt.Errorf("unsupported service type: %s", reflect.TypeOf(*new(T)).String())
	}

	return m.services[creds], nil
}

// getAuth determines and returns an authenticator for accessing a container registry based on the provided authorization data.
// if both dockerCfg and credentials parameters are filled in, credentials is the priority.
func (m *ServiceManager[T]) createAuthOptions(registryURL, dockerCFG, login, password string) ([]client.Option, error) {
	var opts []client.Option

	switch {
	case login != "":
		opts = append(opts, client.WithLoginPassword(login, password))
		m.logger.Debug("init auth from credentials")
	case dockerCFG != "":
		opt, err := client.WithDockercfg(registryURL, dockerCFG)
		if err != nil {
			return nil, fmt.Errorf("failed to get auth from docker config: %w", err)
		}
		opts = append(opts, opt)
		m.logger.Debug("init auth from docker config")
	default:
		return nil, errors.New("there is no authorization data")
	}

	return opts, nil
}

// PackagesService is the package catalog a PackageRepository points at.
type PackagesService struct {
	*service.BasicService

	client   registry.Client
	services map[string]*PackageService

	logger *log.Logger
}

func NewPackagesService(client registry.Client, logger *log.Logger) *PackagesService {
	return &PackagesService{
		BasicService: service.NewBasicService(packagesServiceName, client, logger),
		client:       client,
		services:     make(map[string]*PackageService),
		logger:       logger,
	}
}

func (s *PackagesService) Package(packageName string) *PackageService {
	if s.services == nil {
		s.services = make(map[string]*PackageService)
	}

	if _, exists := s.services[packageName]; !exists {
		s.services[packageName] = NewPackageService(s.client.WithSegment(packageName), s.logger)
	}

	return s.services[packageName]
}

// PackageService addresses one package in the repository.
//
// A PackageRepository serves both shapes at once: v1alpha2 packages publish
// their releases under version/, legacy v1alpha1 modules under release/. Those
// are two different sub-trees of the registry library over the same repository,
// which is why this type carries both.
type PackageService struct {
	*service.BasicService

	client registry.Client

	packageVersion *PackageVersionService
	packageRelease *PackageReleaseService

	logger *log.Logger
}

func NewPackageService(client registry.Client, logger *log.Logger) *PackageService {
	basic := service.NewBasicService(packageServiceName, client, logger)

	return &PackageService{
		BasicService:   basic,
		client:         client,
		packageVersion: &PackageVersionService{VersionService: packages.New(basic).Versions()},
		packageRelease: &PackageReleaseService{ReleaseService: module.New(basic).Releases()},
		logger:         logger,
	}
}

// Versions returns the service for accessing <package>/version path (new v1alpha2 modules).
func (s *PackageService) Versions() *PackageVersionService {
	return s.packageVersion
}

// Release returns the service for accessing <package>/release path (legacy v1alpha1 modules).
func (s *PackageService) Release() *PackageReleaseService {
	return s.packageRelease
}

// GetRoot gets path of the registry root
func (s *PackageService) GetRoot() string {
	return s.client.GetRegistry()
}

// PackageVersionService is <package>/version, the v1alpha2 release repository.
type PackageVersionService struct {
	*packages.VersionService
}

// PackageReleaseService is <package>/release, the legacy v1alpha1 release
// repository. Legacy modules keep the same layout as a module release, so it is
// the module sub-tree that describes it.
type PackageReleaseService struct {
	*module.ReleaseService
}

// packageDefinitionFileAlt is the alternative spelling some builds emit.
const packageDefinitionFileAlt = "package.yml"

// PackageDefinition represents the minimal parsed content of package.yaml.
// It's needed for fallback type detection if the package type label is not set in both version and release images for some reason.
type PackageDefinition struct {
	Type string `yaml:"type"`
}

// ReadPackageDefinition reads package.yaml from the version image and parses its type field.
// It's needed if for some reason we haven't set the package type label in both version and release images.
//
// Returns nil if package.yaml is not found or the image does not exist, and an
// empty definition when the file is present but cannot be parsed — the caller
// tells those apart to distinguish "too old to carry a type" from "carries an
// unusable one".
func (s *PackageVersionService) ReadPackageDefinition(ctx context.Context, tag string) (*PackageDefinition, error) {
	rel, err := s.Fetch(ctx, tag)
	if err != nil {
		if errors.Is(err, dhregistry.ErrImageNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("get version image: %w", err)
	}

	raw, ok := rel.File(definition.PackageFile)
	if !ok {
		raw, ok = rel.File(packageDefinitionFileAlt)
	}

	if !ok {
		return nil, nil
	}

	def, err := definition.ParsePackage(raw)
	if err != nil {
		s.Entry(tag).Warn("failed to parse package.yaml", log.Err(err))

		return &PackageDefinition{}, nil
	}

	return &PackageDefinition{Type: def.Type}, nil
}
