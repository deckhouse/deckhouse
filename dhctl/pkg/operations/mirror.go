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

package operations

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/mirror"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
)

func MirrorDeckhouseToLocalFS(
	mirrorCtx *mirror.Context,
	versions []semver.Version,
) error {
	log.InfoF("Fetching Deckhouse modules list...\t")
	modules, err := mirror.GetDeckhouseExternalModules(mirrorCtx)
	if err != nil {
		return fmt.Errorf("get Deckhouse modules: %w", err)
	}
	log.InfoLn("✅")

	log.InfoF("Creating OCI Image Layouts...\t")
	layouts, err := mirror.CreateOCIImageLayoutsForDeckhouse(mirrorCtx.UnpackedImagesPath, modules)
	if err != nil {
		return fmt.Errorf("create OCI Image Layouts: %w", err)
	}
	log.InfoLn("✅")

	mirror.FillLayoutsImages(mirrorCtx, layouts, versions)

	log.InfoF("Searching for Deckhouse modules images...\t")
	if err = mirror.FindDeckhouseModulesImages(mirrorCtx, layouts); err != nil {
		return fmt.Errorf("find Deckhouse modules images: %w", err)
	}
	log.InfoLn("✅")

	if err = mirror.PullInstallers(mirrorCtx, layouts); err != nil {
		return fmt.Errorf("pull installers: %w", err)
	}

	log.InfoF("Searching for Deckhouse modules digests...\t")
	for imageTag := range layouts.InstallImages {
		digests, err := mirror.ExtractImageDigestsFromDeckhouseInstaller(mirrorCtx, imageTag, layouts.Install)
		if err != nil {
			return fmt.Errorf("extract images digests: %w", err)
		}
		maputil.Join(layouts.DeckhouseImages, digests)
	}
	log.InfoLn("✅")

	if err = mirror.PullDeckhouseReleaseChannels(mirrorCtx, layouts); err != nil {
		return fmt.Errorf("pull release channels: %w", err)
	}
	if err = mirror.PullDeckhouseImages(mirrorCtx, layouts); err != nil {
		return fmt.Errorf("pull Deckhouse: %w", err)
	}

	if err = mirror.PullModules(mirrorCtx, layouts); err != nil {
		return fmt.Errorf("pull Deckhouse modules: %w", err)
	}

	if err = validateLayoutsIfRequired(layouts, mirrorCtx.ValidationMode); err != nil {
		return err
	}

	return nil
}

func validateLayoutsIfRequired(layouts *mirror.ImageLayouts, validationMode mirror.ValidationMode) error {
	layoutsPaths := []layout.Path{layouts.Deckhouse, layouts.ReleaseChannel, layouts.Install}
	for _, moduleImageLayout := range layouts.Modules {
		layoutsPaths = append(layoutsPaths, moduleImageLayout.ModuleLayout)
		layoutsPaths = append(layoutsPaths, moduleImageLayout.ReleasesLayout)
	}
	if err := mirror.ValidateLayouts(layoutsPaths, validationMode); err != nil {
		return fmt.Errorf("OCI Image Layouts validation failure: %w", err)
	}
	return nil
}

func PushDeckhouseToRegistry(mirrorCtx *mirror.Context) error {
	log.InfoF("Find Deckhouse images to push...\t")
	ociLayouts, modulesList, err := findLayoutsToPush(mirrorCtx)
	if err != nil {
		return fmt.Errorf("Find OCI Image Layouts to push: %w", err)
	}
	log.InfoLn("✅")

	log.InfoF("Validating downloaded Deckhouse images...\t")
	if err = mirror.ValidateLayouts(maputil.Values(ociLayouts), mirrorCtx.ValidationMode); err != nil {
		return fmt.Errorf("OCI Image Layouts are invalid: %w", err)
	}
	log.InfoLn("✅")

	refOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptionsFromMirrorContext(mirrorCtx)

	for originalRepo, ociLayout := range ociLayouts {
		log.InfoLn("Mirroring", originalRepo)
		index, err := ociLayout.ImageIndex()
		if err != nil {
			return fmt.Errorf("read image index from %s: %w", ociLayout, err)
		}

		indexManifest, err := index.IndexManifest()
		if err != nil {
			return fmt.Errorf("read index manifest: %w", err)
		}

		repo := strings.Replace(originalRepo, mirrorCtx.DeckhouseRegistryRepo, mirrorCtx.RegistryHost+mirrorCtx.RegistryPath, 1)
		pushCount := 1
		for _, manifest := range indexManifest.Manifests {
			tag := manifest.Annotations["io.deckhouse.image.short_tag"]
			imageRef := repo + ":" + tag

			log.InfoF("[%d / %d] Pushing image %s...\t", pushCount, len(indexManifest.Manifests), imageRef)
			img, err := index.Image(manifest.Digest)
			if err != nil {
				return fmt.Errorf("read image: %w", err)
			}

			ref, err := name.ParseReference(imageRef, refOpts...)
			if err != nil {
				return fmt.Errorf("parse oci layout reference: %w", err)
			}
			if err = remote.Write(ref, img, remoteOpts...); err != nil {
				return fmt.Errorf("write %s to registry: %w", ref.String(), err)
			}
			log.InfoLn("✅")
			pushCount++
		}
		log.InfoF("Repo %s is mirrored ✅\n", originalRepo)
	}

	log.InfoLn("All repositories are mirrored ✅")

	if len(modulesList) == 0 {
		return nil
	}

	log.InfoLn("Pushing modules tags...")
	if err = pushModulesTags(mirrorCtx, modulesList); err != nil {
		return fmt.Errorf("Push modules tags: %w", err)
	}
	log.InfoF("All modules tags are pushed ✅\n")

	return nil
}

func pushModulesTags(mirrorCtx *mirror.Context, modulesList []string) error {
	if len(modulesList) == 0 {
		return nil
	}

	refOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptionsFromMirrorContext(mirrorCtx)
	modulesRepo := path.Join(mirrorCtx.RegistryHost, mirrorCtx.RegistryPath, "modules")
	pushCount := 1
	for _, moduleName := range modulesList {
		log.InfoF("[%d / %d] Pushing module tag for %s...\t", pushCount, len(modulesList), moduleName)

		imageRef, err := name.ParseReference(modulesRepo+":"+moduleName, refOpts...)
		if err != nil {
			return fmt.Errorf("Parse image reference: %w", err)
		}

		img, err := random.Image(32, 1)
		if err != nil {
			return fmt.Errorf("random.Image: %w", err)
		}

		if err = remote.Write(imageRef, img, remoteOpts...); err != nil {
			return fmt.Errorf("Write module index tag: %w", err)
		}
		log.InfoLn("✅")
		pushCount++
	}
	return nil
}

func findLayoutsToPush(mirrorCtx *mirror.Context) (map[string]layout.Path, []string, error) {
	deckhouseIndexRef := mirrorCtx.RegistryHost + mirrorCtx.RegistryPath
	installersIndexRef := filepath.Join(deckhouseIndexRef, "install")
	releasesIndexRef := filepath.Join(deckhouseIndexRef, "release-channel")

	deckhouseLayoutPath := mirrorCtx.UnpackedImagesPath
	installersLayoutPath := filepath.Join(mirrorCtx.UnpackedImagesPath, "install")
	releasesLayoutPath := filepath.Join(mirrorCtx.UnpackedImagesPath, "release-channel")

	deckhouseLayout, err := layout.FromPath(deckhouseLayoutPath)
	if err != nil {
		return nil, nil, err
	}
	installersLayout, err := layout.FromPath(installersLayoutPath)
	if err != nil {
		return nil, nil, err
	}
	releasesLayout, err := layout.FromPath(releasesLayoutPath)
	if err != nil {
		return nil, nil, err
	}

	modulesPath := filepath.Join(mirrorCtx.UnpackedImagesPath, "modules")
	ociLayouts := map[string]layout.Path{
		deckhouseIndexRef:  deckhouseLayout,
		installersIndexRef: installersLayout,
		releasesIndexRef:   releasesLayout,
	}

	modulesNames := make([]string, 0)
	dirs, err := os.ReadDir(modulesPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ociLayouts, []string{}, nil
	case err != nil:
		return nil, nil, err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		moduleName := dir.Name()
		modulesNames = append(modulesNames, moduleName)
		moduleRef := filepath.Join(mirrorCtx.RegistryHost+mirrorCtx.RegistryPath, "modules", moduleName)
		moduleReleasesRef := filepath.Join(mirrorCtx.DeckhouseRegistryRepo, "modules", moduleName, "release")
		moduleLayout, err := layout.FromPath(filepath.Join(modulesPath, moduleName))
		if err != nil {
			return nil, nil, fmt.Errorf("create module layout from path: %w", err)
		}
		moduleReleaseLayout, err := layout.FromPath(filepath.Join(modulesPath, moduleName, "release"))
		if err != nil {
			return nil, nil, fmt.Errorf("create module release layout from path: %w", err)
		}
		ociLayouts[moduleRef] = moduleLayout
		ociLayouts[moduleReleasesRef] = moduleReleaseLayout
	}
	return ociLayouts, modulesNames, nil
}
