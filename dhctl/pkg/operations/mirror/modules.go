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
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/maputil"
)

type Module struct {
	Name         string
	RegistryPath string
	Releases     []string
}

func GetDeckhouseExternalModules(mirrorCtx *Context) ([]Module, error) {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptionsFromMirrorContext(mirrorCtx)
	repoPathBuildFuncForDeckhouseModule := func(repo, moduleName string) string {
		return fmt.Sprintf("%s/modules/%s", mirrorCtx.DeckhouseRegistryRepo, moduleName)
	}

	result, err := getModulesForRepo(
		mirrorCtx.DeckhouseRegistryRepo+"/modules",
		repoPathBuildFuncForDeckhouseModule,
		nameOpts,
		remoteOpts,
	)
	if err != nil {
		return nil, fmt.Errorf("Get Deckhouse modules: %w", err)
	}

	return result, nil
}

func GetExternalModulesFromRepo(repo string, registryAuth authn.Authenticator, insecure, skipVerifyTLS bool) ([]Module, error) {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(registryAuth, insecure, skipVerifyTLS)
	repoPathBuildFuncForExternalModule := func(repo, moduleName string) string {
		return fmt.Sprintf("%s/%s", repo, moduleName)
	}

	result, err := getModulesForRepo(repo, repoPathBuildFuncForExternalModule, nameOpts, remoteOpts)
	if err != nil {
		return nil, fmt.Errorf("Get external modules: %w", err)
	}

	return result, nil
}

func getModulesForRepo(
	repo string,
	repoPathBuildFunc func(repo, moduleName string) string,
	nameOpts []name.Option,
	remoteOpts []remote.Option,
) ([]Module, error) {
	modulesRepo, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("Parsing modules repo: %v", err)
	}

	modules, err := remote.List(modulesRepo, remoteOpts...)
	if err != nil {
		if isRepoNotFoundError(err) {
			return []Module{}, nil
		}
		return nil, fmt.Errorf("Get Deckhouse modules list from %s: %w", repo, err)
	}

	result := make([]Module, 0, len(modules))
	for _, module := range modules {
		m := Module{
			Name:         module,
			RegistryPath: repoPathBuildFunc(repo, module),
			Releases:     []string{},
		}

		repo, err := name.NewRepository(m.RegistryPath+"/release", nameOpts...)
		if err != nil {
			return nil, fmt.Errorf("Parsing repo: %v", err)
		}
		m.Releases, err = remote.List(repo, remoteOpts...)
		if err != nil {
			return nil, fmt.Errorf("Get releases for module %q: %w", m.RegistryPath, err)
		}
		result = append(result, m)
	}
	return result, nil
}

func FindExternalModuleImages(mod *Module, authProvider authn.Authenticator, insecure, skipVerifyTLS bool) (moduleImages, releaseImages map[string]struct{}, err error) {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure, skipVerifyTLS)

	moduleImages = map[string]struct{}{}
	releaseImages, err = getAvailableReleaseChannelsImagesForModule(mod, nameOpts, remoteOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("Get available release channels of module: %w", err)
	}

	releaseChannelVersions, err := fetchVersionsFromModuleReleaseChannels(releaseImages, authProvider, insecure, skipVerifyTLS)
	if err != nil {
		return nil, nil, fmt.Errorf("Fetch versions from %q release channels: %w", mod.Name, err)
	}
	for _, versionTag := range releaseChannelVersions {
		moduleImages[mod.RegistryPath+":"+versionTag] = struct{}{}
		releaseImages[mod.RegistryPath+"/release:"+versionTag] = struct{}{}
	}

	fetchDigestsFrom := maputil.Keys(moduleImages)

	for _, imageTag := range fetchDigestsFrom {
		ref, err := name.ParseReference(imageTag, nameOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("Get digests for %q version: %w", imageTag, err)
		}

		img, err := remote.Image(ref, remoteOpts...)
		if err != nil {
			if isImageNotFoundError(err) {
				continue
			}
			return nil, nil, fmt.Errorf("Get digests for %q version: %w", imageTag, err)
		}

		imagesDigestsJSON, err := readFileFromImage(img, "images_digests.json")
		if err != nil {
			return nil, nil, fmt.Errorf("Get digests for %q version: %w", imageTag, err)
		}

		digests := digestRegex.FindAllString(imagesDigestsJSON.String(), -1)
		for _, digest := range digests {
			moduleImages[mod.RegistryPath+"@"+digest] = struct{}{}
		}
	}

	return moduleImages, releaseImages, nil
}

func getAvailableReleaseChannelsImagesForModule(mod *Module, refOpts []name.Option, remoteOpts []remote.Option) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	for _, imageTag := range []string{
		mod.RegistryPath + "/release:alpha",
		mod.RegistryPath + "/release:beta",
		mod.RegistryPath + "/release:early-access",
		mod.RegistryPath + "/release:stable",
		mod.RegistryPath + "/release:rock-solid",
	} {
		imageRef, err := name.ParseReference(imageTag, refOpts...)
		if err != nil {
			return nil, fmt.Errorf("Parse release channel reference: %w", err)
		}

		_, err = remote.Head(imageRef, remoteOpts...)
		if err != nil {
			if isImageNotFoundError(err) {
				continue
			}
			return nil, fmt.Errorf("Check if release channel is present: %w", err)
		}
		result[imageTag] = struct{}{}
	}

	return result, nil
}
