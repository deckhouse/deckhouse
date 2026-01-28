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

	v1 "github.com/google/go-containerregistry/pkg/v1"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"

	dhRelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type deckhouseReleaseService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option

	logger *log.Logger
}

func newDeckhouseReleaseService(registryAddress string, registryConfig *utils.RegistryConfig, logger *log.Logger) *deckhouseReleaseService {
	return &deckhouseReleaseService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: utils.GenerateRegistryOptions(registryConfig, logger),
		logger:          logger,
	}
}

func (svc *deckhouseReleaseService) GetDeckhouseRelease(ctx context.Context, releaseChannel string) (*dhRelease.ReleaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, "release-channel"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		return nil, fmt.Errorf("fetch image: %w", err)
	}

	// TODO: see check_release with same method
	releaseMetadata, err := svc.fetchReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}

	if releaseMetadata.Version == "" {
		return nil, fmt.Errorf("release metadata malformed: no version found")
	}

	return releaseMetadata, nil
}

func (svc *deckhouseReleaseService) ListDeckhouseReleases(ctx context.Context) ([]string, error) {
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

func (svc *deckhouseReleaseService) fetchReleaseMetadata(img v1.Image) (*dhRelease.ReleaseMetadata, error) {
	var meta = new(dhRelease.ReleaseMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	err = rr.untarMetadata(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
	}

	if rr.moduleReader.Len() > 0 {
		var ModuleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&ModuleDefinition)
		if err != nil {
			return nil, fmt.Errorf("unmarshal module yaml failed: %w", err)
		}

		meta.ModuleDefinition = &ModuleDefinition
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			svc.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}
