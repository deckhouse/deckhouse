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
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Masterminds/semver/v3"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type moduleReleaseService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option

	logger *log.Logger
}

func newModuleReleaseService(registryAddress string, registryConfig *utils.RegistryConfig, logger *log.Logger) *moduleReleaseService {
	return &moduleReleaseService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: utils.GenerateRegistryOptions(registryConfig, logger),
		logger:          logger,
	}
}

func (svc *moduleReleaseService) ListModules(ctx context.Context) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(svc.registry, svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, nil
}

var (
	ErrChannelIsNotFound = errors.New("channel is not found")
	ErrModuleIsNotFound  = errors.New("module is not found")
)

func (svc *moduleReleaseService) ListModuleTags(ctx context.Context, moduleName string) ([]string, error) {
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

func (svc *moduleReleaseService) GetModuleRelease(ctx context.Context, moduleName, releaseChannel string) (*ModuleReleaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	// The same release image the controller and dhctl read, read by the same code: a third
	// copy of the tar walk here is a third place to fix whenever one of them changes. Only
	// the semver parse is this surface's own - cr keeps the version opaque because a dev
	// build ships one that is not a semver at all.
	info, err := cr.ResolveChannel(ctx, regCli, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		return nil, fmt.Errorf("fetch module release metadata error: %w", err)
	}

	version, err := semver.NewVersion(info.Version)
	if err != nil {
		return nil, fmt.Errorf("module release %q metadata malformed: parse version %q: %w", moduleName, info.Version, err)
	}

	meta := &ModuleReleaseMetadata{Version: version, Changelog: info.Changelog}

	if len(info.ModuleYAML) > 0 {
		definition := new(moduletypes.Definition)
		if err = yaml.Unmarshal(info.ModuleYAML, definition); err != nil {
			return nil, fmt.Errorf("unmarshal module yaml failed: %w", err)
		}

		meta.ModuleDefinition = definition
	}

	return meta, nil
}

// ModuleReleaseMetadata is what this service reads out of a module release image. It lives
// here rather than in the downloader package because this is the only consumer left: the
// downloader itself resolves releases through go_lib/dependency/cr, which keeps the version
// as an opaque string, while the d8 CLI surface below still speaks semver.
type ModuleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog        map[string]any          `json:"-"`
	ModuleDefinition *moduletypes.Definition `json:"module,omitempty"`
}
