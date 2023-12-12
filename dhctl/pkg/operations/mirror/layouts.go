// Copyright 2023 Flant JSC
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

package mirror

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
)

type ImageLayouts struct {
	Deckhouse       layout.Path
	DeckhouseImages map[string]struct{}

	Install       layout.Path
	InstallImages map[string]struct{}

	ReleaseChannel       layout.Path
	ReleaseChannelImages map[string]struct{}

	Modules map[string]ModuleImageLayout
}

type ModuleImageLayout struct {
	ModuleLayout layout.Path
	ModuleImages map[string]struct{}

	ReleasesLayout layout.Path
	ReleaseImages  map[string]struct{}
}

func CreateOCIImageLayoutsForDeckhouse(
	registryRepo string,
	rootFolder string,
	modules []Module,
) (*ImageLayouts, error) {
	var err error
	layouts := &ImageLayouts{Modules: map[string]ModuleImageLayout{}}

	fsPaths := map[*layout.Path]string{
		&layouts.Deckhouse:      filepath.Join(rootFolder, registryRepo),
		&layouts.Install:        filepath.Join(rootFolder, registryRepo, "install"),
		&layouts.ReleaseChannel: filepath.Join(rootFolder, registryRepo, "release-channel"),
	}
	for layoutPtr, fsPath := range fsPaths {
		*layoutPtr, err = CreateEmptyImageLayoutAtPath(fsPath)
		if err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", fsPath, err)
		}
	}

	for _, module := range modules {
		path := filepath.Join(rootFolder, registryRepo, "modules", module.Name)
		moduleLayout, err := CreateEmptyImageLayoutAtPath(path)
		if err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", path, err)
		}

		path = filepath.Join(rootFolder, registryRepo, "modules", module.Name, "release")
		moduleReleasesLayout, err := CreateEmptyImageLayoutAtPath(path)
		if err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", path, err)
		}

		layouts.Modules[module.Name] = ModuleImageLayout{
			ModuleLayout:   moduleLayout,
			ModuleImages:   map[string]struct{}{},
			ReleasesLayout: moduleReleasesLayout,
			ReleaseImages:  map[string]struct{}{},
		}
	}

	return layouts, nil
}

func CreateEmptyImageLayoutAtPath(path string) (layout.Path, error) {
	layoutFilePath := filepath.Join(path, "oci-layout")
	indexFilePath := filepath.Join(path, "index.json")
	blobsPath := filepath.Join(path, "blobs")

	if err := os.MkdirAll(blobsPath, 0755); err != nil {
		return "", fmt.Errorf("mkdir for blobs: %w", err)
	}

	layoutContents := ociLayout{ImageLayoutVersion: "1.0.0"}
	indexContents := indexSchema{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
	}

	rawJSON, err := json.MarshalIndent(indexContents, "", "    ")
	if err != nil {
		return "", fmt.Errorf("create index.json: %w", err)
	}
	if err = os.WriteFile(indexFilePath, rawJSON, 0644); err != nil {
		return "", fmt.Errorf("create index.json: %w", err)
	}

	rawJSON, err = json.MarshalIndent(layoutContents, "", "    ")
	if err != nil {
		return "", fmt.Errorf("create oci-layout: %w", err)
	}
	if err = os.WriteFile(layoutFilePath, rawJSON, 0644); err != nil {
		return "", fmt.Errorf("create oci-layout: %w", err)
	}

	return layout.Path(path), nil
}

type indexSchema struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Manifests     []struct {
		MediaType string `json:"mediaType,omitempty"`
		Size      int    `json:"size,omitempty"`
		Digest    string `json:"digest,omitempty"`
	} `json:"manifests"`
}

type ociLayout struct {
	ImageLayoutVersion string `json:"imageLayoutVersion"`
}

func FillLayoutsImages(mirrorCtx *Context, layouts *ImageLayouts, deckhouseVersions []*semver.Version) {
	layouts.DeckhouseImages = map[string]struct{}{
		mirrorCtx.DeckhouseRegistryRepo + ":alpha":        {},
		mirrorCtx.DeckhouseRegistryRepo + ":beta":         {},
		mirrorCtx.DeckhouseRegistryRepo + ":early-access": {},
		mirrorCtx.DeckhouseRegistryRepo + ":stable":       {},
		mirrorCtx.DeckhouseRegistryRepo + ":rock-solid":   {},
	}

	layouts.InstallImages = map[string]struct{}{
		mirrorCtx.DeckhouseRegistryRepo + "/install:alpha":        {},
		mirrorCtx.DeckhouseRegistryRepo + "/install:beta":         {},
		mirrorCtx.DeckhouseRegistryRepo + "/install:early-access": {},
		mirrorCtx.DeckhouseRegistryRepo + "/install:stable":       {},
		mirrorCtx.DeckhouseRegistryRepo + "/install:rock-solid":   {},
	}

	layouts.ReleaseChannelImages = map[string]struct{}{
		mirrorCtx.DeckhouseRegistryRepo + "/release-channel:alpha":        {},
		mirrorCtx.DeckhouseRegistryRepo + "/release-channel:beta":         {},
		mirrorCtx.DeckhouseRegistryRepo + "/release-channel:early-access": {},
		mirrorCtx.DeckhouseRegistryRepo + "/release-channel:stable":       {},
		mirrorCtx.DeckhouseRegistryRepo + "/release-channel:rock-solid":   {},
	}

	for _, version := range deckhouseVersions {
		layouts.DeckhouseImages[fmt.Sprintf("%s:v%s", mirrorCtx.DeckhouseRegistryRepo, version.String())] = struct{}{}
		layouts.InstallImages[fmt.Sprintf("%s/install:v%s", mirrorCtx.DeckhouseRegistryRepo, version.String())] = struct{}{}
		layouts.ReleaseChannelImages[fmt.Sprintf("%s/release-channel:v%s", mirrorCtx.DeckhouseRegistryRepo, version.String())] = struct{}{}
	}
}

var digestRegex = regexp.MustCompile(`sha256:([a-f0-9]{64})`)

func FindDeckhouseModulesImages(mirrorCtx *Context, layouts *ImageLayouts) error {
	modulesNames := maputil.Keys(layouts.Modules)
	for _, moduleName := range modulesNames {
		moduleData := layouts.Modules[moduleName]
		moduleData.ReleaseImages = map[string]struct{}{
			mirrorCtx.DeckhouseRegistryRepo + "/modules/" + moduleName + "/release:alpha":        {},
			mirrorCtx.DeckhouseRegistryRepo + "/modules/" + moduleName + "/release:beta":         {},
			mirrorCtx.DeckhouseRegistryRepo + "/modules/" + moduleName + "/release:early-access": {},
			mirrorCtx.DeckhouseRegistryRepo + "/modules/" + moduleName + "/release:stable":       {},
			mirrorCtx.DeckhouseRegistryRepo + "/modules/" + moduleName + "/release:rock-solid":   {},
		}

		channelVersions, err := fetchVersionsFromModuleReleaseChannels(moduleData.ReleaseImages, mirrorCtx.RegistryAuth, mirrorCtx.Insecure)
		if err != nil {
			return fmt.Errorf("fetch versions from %q release channels: %w", moduleName, err)
		}

		for _, moduleVersion := range channelVersions {
			moduleData.ModuleImages[mirrorCtx.DeckhouseRegistryRepo+"/modules/"+moduleName+":"+moduleVersion] = struct{}{}
			moduleData.ReleaseImages[mirrorCtx.DeckhouseRegistryRepo+"/modules/"+moduleName+"/release:"+moduleVersion] = struct{}{}
		}

		nameOpts, remoteOpts := MakeRemoteRegistryRequestOptionsFromMirrorContext(mirrorCtx)
		fetchDigestsFrom := maputil.Clone(moduleData.ModuleImages)
		for imageTag := range fetchDigestsFrom {
			ref, err := name.ParseReference(imageTag, nameOpts...)
			if err != nil {
				return fmt.Errorf("get digests for %q version: %w", imageTag, err)
			}

			img, err := remote.Image(ref, remoteOpts...)
			if err != nil {
				return fmt.Errorf("get digests for %q version: %w", imageTag, err)
			}

			imagesDigestsJSON, err := readFileFromImage(img, "images_digests.json")
			if err != nil {
				return fmt.Errorf("get digests for %q version: %w", imageTag, err)
			}

			digests := digestRegex.FindAllString(imagesDigestsJSON.String(), -1)
			for _, digest := range digests {
				moduleData.ModuleImages[mirrorCtx.DeckhouseRegistryRepo+"/modules/"+moduleName+"@"+digest] = struct{}{}
			}
		}

		layouts.Modules[moduleName] = moduleData
	}

	return nil
}

func fetchVersionsFromModuleReleaseChannels(
	releaseChannelImages map[string]struct{},
	authProvider authn.Authenticator,
	insecure bool,
) (map[string]string, error) {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure)
	channelVersions := map[string]string{}
	for imageTag := range releaseChannelImages {

		ref, err := name.ParseReference(imageTag, nameOpts...)
		if err != nil {
			return nil, fmt.Errorf("pull %q release channel: %w", imageTag, err)
		}

		img, err := remote.Image(ref, remoteOpts...)
		if err != nil {
			if isImageNotFoundError(err) {
				continue
			}
			return nil, fmt.Errorf("pull %q release channel: %w", imageTag, err)
		}

		versionJSON, err := readFileFromImage(img, "version.json")
		if err != nil {
			return nil, fmt.Errorf("read version.json from %q: %w", imageTag, err)
		}

		tmp := &struct {
			Version string `json:"version"`
		}{}
		if err = json.Unmarshal(versionJSON.Bytes(), tmp); err != nil {
			return nil, fmt.Errorf("parse version.json: %w", err)
		}

		channelVersions[imageTag] = tmp.Version
	}

	return channelVersions, nil
}

func isImageNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "MANIFEST_UNKNOWN") || strings.Contains(errMsg, "404 Not Found")
}
