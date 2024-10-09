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
	"encoding/json"
	"fmt"
	"path"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/ettle/strcase"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"gopkg.in/yaml.v2"
)

type Service struct {
	dc                   dependency.Container
	downloadedModulesDir string

	ms              *v1alpha1.ModuleSource
	registryOptions []cr.Option
}

func NewService(dc dependency.Container, downloadedModulesDir string, ms *v1alpha1.ModuleSource, registryOptions []cr.Option) *Service {
	return &Service{
		dc:                   dc,
		downloadedModulesDir: downloadedModulesDir,
		ms:                   ms,
		registryOptions:      registryOptions,
	}
}

// DownloadMetadataFromReleaseChannel downloads only module release image with metadata: version.json, checksum.json(soon)
// does not fetch and install the desired version on the module, only fetches its module definition
func (svc *Service) DownloadMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) error {
	moduleVersion, checksum, changelog, err := svc.fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum)
	if err != nil {
		return err
	}

	fmt.Printf("%s,%s,%s\n fetched: %s,%s,%s\n\n", moduleName, releaseChannel, moduleChecksum, moduleVersion, checksum, changelog)

	// img, err := svc.fetchImage(moduleName, moduleVersion)
	// if err != nil {
	// 	return res, err
	// }

	// def, err := svc.fetchModuleDefinitionFromImage(moduleName, img)
	// if err != nil {
	// 	return res, err
	// }

	return nil
}

func (svc *Service) fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) (
	/* moduleVersion */ string /*newChecksum*/, string /*changelog*/, map[string]any, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.ms.Spec.Registry.Repo, moduleName, "release"), svc.registryOptions...)
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch image error: %v", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch digest error: %v", err)
	}

	if moduleChecksum == digest.String() {
		return "", moduleChecksum, nil, nil
	}

	moduleMetadata, err := svc.fetchModuleReleaseMetadata(img)
	if err != nil {
		return "", digest.String(), nil, fmt.Errorf("fetch release metadata error: %v", err)
	}

	if moduleMetadata.Version == nil {
		return "", digest.String(), nil, fmt.Errorf("module %q metadata malformed: no version found", moduleName)
	}

	return "v" + moduleMetadata.Version.String(), digest.String(), moduleMetadata.Changelog, nil
}

type moduleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog map[string]any
}

func (svc *Service) fetchModuleReleaseMetadata(img v1.Image) (moduleReleaseMetadata, error) {
	var meta moduleReleaseMetadata

	rc := mutate.Extract(img)
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := rr.untarMetadata(rc)
	if err != nil {
		return meta, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return meta, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			meta.Changelog = make(map[string]any)
			return meta, nil
		}
		meta.Changelog = changelog
	}

	return meta, err
}
