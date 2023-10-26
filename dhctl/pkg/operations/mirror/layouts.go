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

	"github.com/Masterminds/semver/v3"
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

func CreateOCIImageLayouts(
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
		if err := createEmptyImageLayoutAtPath(fsPath); err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", fsPath, err)
		}
		*layoutPtr, err = layout.FromPath(fsPath)
		if err != nil {
			return nil, fmt.Errorf("get OCI Image Layout from %s: %w", err)
		}
	}

	for _, module := range modules {
		path := filepath.Join(rootFolder, registryRepo, "modules", module.Name)
		if err := createEmptyImageLayoutAtPath(path); err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", path, err)
		}
		moduleLayout, err := layout.FromPath(path)
		if err != nil {
			return nil, fmt.Errorf("get OCI Image Layout from %s: %w", err)
		}

		path = filepath.Join(rootFolder, registryRepo, "modules", module.Name, "release")
		if err := createEmptyImageLayoutAtPath(path); err != nil {
			return nil, fmt.Errorf("create OCI Image Layout at %s: %w", path, err)
		}
		moduleReleasesLayout, err := layout.FromPath(path)
		if err != nil {
			return nil, fmt.Errorf("get OCI Image Layout from %s: %w", err)
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

func createEmptyImageLayoutAtPath(path string) error {
	layoutFilePath := filepath.Join(path, "oci-layout")
	indexFilePath := filepath.Join(path, "index.json")
	blobsPath := filepath.Join(path, "blobs")

	if err := os.MkdirAll(blobsPath, 0755); err != nil {
		return fmt.Errorf("mkdir for blobs: %w", err)
	}

	layoutContents := ociLayout{ImageLayoutVersion: "1.0.0"}
	indexContents := indexSchema{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
	}

	rawJSON, err := json.MarshalIndent(indexContents, "", "    ")
	if err != nil {
		return fmt.Errorf("create index.json: %w", err)
	}
	if err = os.WriteFile(indexFilePath, rawJSON, 0644); err != nil {
		return fmt.Errorf("create index.json: %w", err)
	}

	rawJSON, err = json.MarshalIndent(layoutContents, "", "    ")
	if err != nil {
		return fmt.Errorf("create oci-layout: %w", err)
	}
	if err = os.WriteFile(layoutFilePath, rawJSON, 0644); err != nil {
		return fmt.Errorf("create oci-layout: %w", err)
	}

	return nil
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
		mirrorCtx.RegistryRepo + ":alpha":        {},
		mirrorCtx.RegistryRepo + ":beta":         {},
		mirrorCtx.RegistryRepo + ":early-access": {},
		mirrorCtx.RegistryRepo + ":stable":       {},
		mirrorCtx.RegistryRepo + ":rock-solid":   {},
	}

	layouts.InstallImages = map[string]struct{}{
		mirrorCtx.RegistryRepo + "/install:alpha":        {},
		mirrorCtx.RegistryRepo + "/install:beta":         {},
		mirrorCtx.RegistryRepo + "/install:early-access": {},
		mirrorCtx.RegistryRepo + "/install:stable":       {},
		mirrorCtx.RegistryRepo + "/install:rock-solid":   {},
	}

	layouts.ReleaseChannelImages = map[string]struct{}{
		mirrorCtx.RegistryRepo + "/release-channel:alpha":        {},
		mirrorCtx.RegistryRepo + "/release-channel:beta":         {},
		mirrorCtx.RegistryRepo + "/release-channel:early-access": {},
		mirrorCtx.RegistryRepo + "/release-channel:stable":       {},
		mirrorCtx.RegistryRepo + "/release-channel:rock-solid":   {},
	}

	for _, version := range deckhouseVersions {
		layouts.DeckhouseImages[fmt.Sprintf("%s:v%s", mirrorCtx.RegistryRepo, version.String())] = struct{}{}
		layouts.InstallImages[fmt.Sprintf("%s/install:v%s", mirrorCtx.RegistryRepo, version.String())] = struct{}{}
		layouts.ReleaseChannelImages[fmt.Sprintf("%s/release-channel:v%s", mirrorCtx.RegistryRepo, version.String())] = struct{}{}
	}
}

var digestRegex = regexp.MustCompile(`sha256:([a-f0-9]{64})`)

func FindDeckhouseModulesImages(mirrorCtx *Context, layouts *ImageLayouts) error {
	modulesNames := maputil.Keys(layouts.Modules)
	for _, moduleName := range modulesNames {
		moduleData := layouts.Modules[moduleName]
		moduleData.ReleaseImages = map[string]struct{}{
			mirrorCtx.RegistryRepo + "/modules/" + moduleName + "/release:alpha":        {},
			mirrorCtx.RegistryRepo + "/modules/" + moduleName + "/release:beta":         {},
			mirrorCtx.RegistryRepo + "/modules/" + moduleName + "/release:early-access": {},
			mirrorCtx.RegistryRepo + "/modules/" + moduleName + "/release:stable":       {},
			mirrorCtx.RegistryRepo + "/modules/" + moduleName + "/release:rock-solid":   {},
		}

		channelVersions, err := fetchVersionsFromModuleReleaseChannels(mirrorCtx, moduleData.ReleaseImages)
		if err != nil {
			return fmt.Errorf("fetch versions from %q release channels: %w", moduleName, err)
		}

		for _, moduleVersion := range channelVersions {
			moduleData.ModuleImages[mirrorCtx.RegistryRepo+"/modules/"+moduleName+":"+moduleVersion] = struct{}{}
			moduleData.ReleaseImages[mirrorCtx.RegistryRepo+"/modules/"+moduleName+"/release:"+moduleVersion] = struct{}{}
		}

		fetchDigestsFrom := maputil.Clone(moduleData.ModuleImages)
		for imageTag := range fetchDigestsFrom {
			nameOpts := []name.Option{}
			remoteOpts := []remote.Option{}
			if mirrorCtx.Insecure {
				nameOpts = append(nameOpts, name.Insecure)
			}
			if mirrorCtx.RegistryAuth != nil {
				remoteOpts = append(remoteOpts, remote.WithAuth(mirrorCtx.RegistryAuth))
			}

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
				moduleData.ModuleImages[mirrorCtx.RegistryRepo+"/modules/"+moduleName+"@"+digest] = struct{}{}
			}
		}

		layouts.Modules[moduleName] = moduleData
	}

	return nil
}

func fetchVersionsFromModuleReleaseChannels(
	mirrorCtx *Context,
	releaseChannelImages map[string]struct{},
) (map[string]string, error) {
	channelVersions := map[string]string{}
	for imageTag := range releaseChannelImages {
		opts := []name.Option{}
		remoteOpts := []remote.Option{}
		if mirrorCtx.Insecure {
			opts = append(opts, name.Insecure)
		}
		if mirrorCtx.RegistryAuth != nil {
			remoteOpts = append(remoteOpts, remote.WithAuth(mirrorCtx.RegistryAuth))
		}

		ref, err := name.ParseReference(imageTag, opts...)
		if err != nil {
			return nil, fmt.Errorf("pull %q release channel: %w", imageTag, err)
		}

		img, err := remote.Image(ref, remoteOpts...)
		if err != nil {
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
