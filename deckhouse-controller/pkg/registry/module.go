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
	"log/slog"
	"path"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v2"

	modRelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
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

	return ls, err
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

// GetModuleRelease with digest-first optimization
func (svc *moduleReleaseService) GetModuleRelease(ctx context.Context, moduleName, releaseChannel string) (*modRelease.ModuleReleaseMetadata, error) {
	// Get registry client
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	// Get current digest from registry (lightweight operation)
	currentDigest, err := regCli.Digest(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		svc.logger.Warn("Failed to get module digest, falling back to full image download",
			slog.String("module", moduleName),
			slog.String("channel", releaseChannel),
			log.Err(err))

		// Fallback to original behavior if digest call fails
		return svc.getModuleReleaseFallback(ctx, regCli, moduleName, releaseChannel)
	}

	// Fetch image and extract metadata
	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}
		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	// Verify digest consistency between calls
	imageDigest, err := img.Digest()
	if err == nil && currentDigest != imageDigest.String() {
		svc.logger.Warn("Module image digest inconsistency between digest and image calls",
			slog.String("digest_call", currentDigest),
			slog.String("image_digest", imageDigest.String()),
			slog.String("module", moduleName),
			slog.String("channel", releaseChannel))
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

// getModuleReleaseFallback implements the original behavior when digest optimization fails
func (svc *moduleReleaseService) getModuleReleaseFallback(ctx context.Context, regCli cr.Client, moduleName, releaseChannel string) (*modRelease.ModuleReleaseMetadata, error) {
	svc.logger.Debug("Using fallback module retrieval method",
		slog.String("module", moduleName),
		slog.String("channel", releaseChannel))

	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
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

func (svc *moduleReleaseService) fetchModuleReleaseMetadata(img v1.Image) (*modRelease.ModuleReleaseMetadata, error) {
	var meta = new(modRelease.ModuleReleaseMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			svc.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			meta.Changelog = make(map[string]any)

			return nil, nil
		}

		meta.Changelog = changelog
	}

	if rr.moduleReader.Len() > 0 {
		var ModuleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&ModuleDefinition)
		if err != nil {
			// if module.yaml decode failed - warn about it but don't fail the release
			svc.logger.Warn("Unmarshal module yaml failed", log.Err(err))

			meta.ModuleDefinition = nil

			return meta, nil
		}

		meta.ModuleDefinition = &ModuleDefinition
	}

	return meta, nil
}
