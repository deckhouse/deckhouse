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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/registry/client"
)

const (
	packageVersionSegment = "version"

	packagesServiceName       = "packages"
	packageServiceName        = "package"
	packageVersionServiceName = "package_version"
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
	credentials string
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
		credentials: config.Credentials,
		ca:          config.CA,
		userAgent:   config.UserAgent,
	}

	// if service with these creds already exists - return it
	_, svcExists := m.services[creds]
	if svcExists {
		return m.services[creds], nil
	}

	auth, err := getAuth(registryURL, config.DockerConfig, config.Credentials) // factory method
	if err != nil {
		return nil, fmt.Errorf("failed to get auth from docker config: %w", err)
	}

	// remove cached service with old credentials for this registryURL
	cachedCreds, isCached := m.cachedCredentials[registryURL]
	if isCached {
		delete(m.services, *cachedCreds)
		m.cachedCredentials[registryURL] = &creds
	}

	c := client.NewClientWithOptions(registryURL, &client.Options{
		Auth:      auth,
		Scheme:    config.Scheme,
		CA:        config.CA,
		UserAgent: config.UserAgent,
		Logger:    m.logger,
	})

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
func getAuth(registryURL, dockerCFG, credentials string) (authn.Authenticator, error) {
	var auth authn.Authenticator
	var err error

	switch {
	case credentials != "":
		if auth, err = client.AuthFromCredentials(registryURL, credentials); err != nil {
			return nil, fmt.Errorf("failed to get auth from credentials: %w", err)
		}
	case dockerCFG != "":
		if auth, err = client.AuthFromDockerConfig(registryURL, dockerCFG); err != nil {
			return nil, fmt.Errorf("failed to get auth from docker config: %w", err)
		}
	default:
		return nil, errors.New("there is no authorization data")
	}

	return auth, err
}

type PackagesService struct {
	client registry.Client

	*BasicService

	services map[string]*PackageService

	logger *log.Logger
}

func NewPackagesService(client registry.Client, logger *log.Logger) *PackagesService {
	return &PackagesService{
		client: client,

		BasicService: NewBasicService(packagesServiceName, client, logger),
		services:     make(map[string]*PackageService),

		logger: logger,
	}
}

func (s *PackagesService) Package(packageName string) *PackageService {
	if s.services == nil {
		s.services = make(map[string]*PackageService)
	}

	if _, exists := s.services[packageName]; !exists {
		packageClient := s.client.WithSegment(packageName)
		s.services[packageName] = NewPackageService(packageClient, s.logger)
	}

	return s.services[packageName]
}

// PackageService provides high-level operations for Deckhouse platform management
type PackageService struct {
	client registry.Client

	*BasicService
	packageVersion *PackageVersionService

	logger *log.Logger
}

// NewPackageService creates a new deckhouse service
func NewPackageService(client registry.Client, logger *log.Logger) *PackageService {
	return &PackageService{
		client: client,

		BasicService:   NewBasicService(packageServiceName, client, logger),
		packageVersion: NewPackageVersionService(NewBasicService(packageVersionServiceName, client.WithSegment(packageVersionSegment), logger)),

		logger: logger,
	}
}

func (s *PackageService) ReleaseChannels() *PackageVersionService {
	return s.packageVersion
}

// GetRoot gets path of the registry root
func (s *PackageService) GetRoot() string {
	return s.client.GetRegistry()
}

type PackageVersionService struct {
	*BasicService
}

func NewPackageVersionService(basicService *BasicService) *PackageVersionService {
	return &PackageVersionService{
		BasicService: basicService,
	}
}

type PackageVersionMetadata struct {
	Version string

	Changelog map[string]interface{}
}

func (s *PackageVersionService) GetMetadata(ctx context.Context, tag string) (*PackageVersionMetadata, error) {
	logger := s.logger.With(slog.String("service", s.name), slog.String("tag", tag))

	logger.Debug("Getting metadata")

	img, err := s.client.GetImage(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	meta, err := s.extractPackageVersionMetadata(img.Extract())
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	return meta, nil
}

type packageVersionStruct struct {
	Version string `json:"version"`
}

func (s *PackageVersionService) extractPackageVersionMetadata(rc io.ReadCloser) (*PackageVersionMetadata, error) {
	var meta = new(PackageVersionMetadata)

	defer rc.Close()

	drr := &packageVersionReader{
		versionReader: bytes.NewBuffer(nil),
	}

	err := drr.untarMetadata(rc)
	if err != nil {
		return nil, err
	}

	var version packageVersionStruct
	if drr.versionReader.Len() > 0 {
		err = json.NewDecoder(drr.versionReader).Decode(&version)
		if err != nil {
			return nil, fmt.Errorf("metadata decode: %w", err)
		}

		meta.Version = version.Version
	}

	if drr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(drr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			s.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}

type packageVersionReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
}

func (rr *packageVersionReader) untarMetadata(rc io.Reader) error {
	tr := tar.NewReader(rc)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}

		if err != nil {
			return err
		}

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rr.versionReader, tr)
			if err != nil {
				return err
			}
		case "changelog.yaml", "changelog.yml":
			_, err = io.Copy(rr.changelogReader, tr)
			if err != nil {
				return err
			}

		default:
			continue
		}
	}
}
