// Copyright 2022 Flant JSC
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/ettle/strcase"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

type ModuleService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option
}

func NewModuleService(registryAddress string, registryConfig *utils.RegistryConfig) *ModuleService {
	return &ModuleService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: utils.GenerateRegistryOptions(registryConfig),
	}
}

func (svc *ModuleService) ListModules(ctx context.Context) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(svc.registry, svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, err
}

var (
	ErrChannelIsNotFound = errors.New("channel is not found")
	ErrModuleIsNotFound  = errors.New("module is not found")
)

func (svc *ModuleService) ListModuleTags(ctx context.Context, moduleName string) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.NameUnknownErrorCode)) {
			err = errors.Join(err, ErrModuleIsNotFound)
		}

		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, err
}

type ModuleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog map[string]any
}

func (svc *ModuleService) GetModuleRelease(moduleName, releaseChannel string) (*ModuleReleaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	moduleMetadata, err := svc.fetchModuleReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch module release metadata error: %w", err)
	}

	if moduleMetadata.Version == nil {
		return nil, fmt.Errorf("module release %q metadata malformed: no version found", moduleName)
	}

	return moduleMetadata, nil
}

func (svc *ModuleService) fetchModuleReleaseMetadata(img v1.Image) (*ModuleReleaseMetadata, error) {
	var meta = new(ModuleReleaseMetadata)

	rc := mutate.Extract(img)
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := rr.untarModuleMetadata(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)

		if err != nil {
			meta.Changelog = make(map[string]any)
			return nil, nil
		}

		meta.Changelog = changelog
	}

	return meta, nil
}
