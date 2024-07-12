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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/mirror"
)

func PullExternalModulesToLocalFS(
	sourceYmlPath, mirrorDirectoryPath, moduleFilterExpression string,
	skipVerifyTLS bool,
) error {
	src, err := loadModuleSourceFromPath(sourceYmlPath)
	if err != nil {
		return fmt.Errorf("Read ModuleSource: %w", err)
	}

	filter := mirror.ParseModuleFilterString(moduleFilterExpression)

	insecure := strings.ToUpper(src.Spec.Registry.Scheme) == "HTTP"
	authProvider, err := findRegistryAuthCredentials(src)
	if err != nil {
		return fmt.Errorf("Parse dockerCfg: %w", err)
	}

	modules, err := mirror.GetExternalModulesFromRepo(src.Spec.Registry.Repo, authProvider, insecure, skipVerifyTLS)
	if err != nil {
		return fmt.Errorf("Get external modules from %q: %w", src.Spec.Registry.Repo, err)
	}

	if len(modules) == 0 {
		log.WarnLn("No modules found in ModuleSource")
		return nil
	}

	tagsResolver := mirror.NewTagsResolver()

	for i, module := range modules {
		if !filter.Match(module) {
			continue
		}

		filter.FilterModuleReleases(&module)

		log.InfoF("[%d / %d] Pulling module %s...\n", i+1, len(modules), module.RegistryPath)

		moduleLayout, err := mirror.CreateEmptyImageLayoutAtPath(filepath.Join(mirrorDirectoryPath, module.Name))
		if err != nil {
			return fmt.Errorf("Create module OCI Layouts: %w", err)
		}
		moduleReleasesLayout, err := mirror.CreateEmptyImageLayoutAtPath(filepath.Join(mirrorDirectoryPath, module.Name, "release"))
		if err != nil {
			return fmt.Errorf("Create module OCI Layouts: %w", err)
		}

		moduleImageSet, releasesImageSet, err := mirror.FindExternalModuleImages(&module, authProvider, filter != nil, insecure, skipVerifyTLS)
		if err != nil {
			return fmt.Errorf("Find external module images`: %w", err)
		}

		for _, imageSet := range []map[string]struct{}{moduleImageSet, releasesImageSet} {
			if err = tagsResolver.ResolveTagsDigestsFromImageSet(imageSet, authProvider, insecure, skipVerifyTLS); err != nil {
				return fmt.Errorf("Resolve digests for images tags: %w", err)
			}
		}

		log.InfoLn("Beginning to pull module contents")
		err = mirror.PullImageSet(authProvider, moduleLayout, moduleImageSet, tagsResolver.GetTagDigest, insecure, skipVerifyTLS, false)
		if err != nil {
			return fmt.Errorf("Pull images: %w", err)
		}
		log.InfoLn("✅ Module contents pulled successfully")

		log.InfoLn("Beginning to pull releases for module")
		err = mirror.PullImageSet(authProvider, moduleReleasesLayout, releasesImageSet, tagsResolver.GetTagDigest, insecure, skipVerifyTLS, false)
		if err != nil {
			return fmt.Errorf("Pull images: %w", err)
		}
		log.InfoLn("✅ Releases for module pulled successfully")
	}

	return nil
}

func loadModuleSourceFromPath(sourceYmlPath string) (*v1alpha1.ModuleSource, error) {
	rawYml, err := os.ReadFile(sourceYmlPath)
	if err != nil {
		return nil, fmt.Errorf("Read %q: %w", sourceYmlPath, err)
	}

	src := &v1alpha1.ModuleSource{}
	if err = yaml.Unmarshal(rawYml, src); err != nil {
		return nil, fmt.Errorf("Parse ModuleSource YAML: %w", err)
	}

	if src.Spec.Registry.Scheme == "" {
		src.Spec.Registry.Scheme = "HTTPS"
	}

	return src, nil
}

func findRegistryAuthCredentials(source *v1alpha1.ModuleSource) (authn.Authenticator, error) {
	buf, err := base64.StdEncoding.DecodeString(source.Spec.Registry.DockerCFG)
	if err != nil {
		return nil, fmt.Errorf("Decode dockerCfg: %w", err)
	}

	registryURL, err := url.Parse(strings.ToLower(source.Spec.Registry.Scheme) + "://" + source.Spec.Registry.Repo)
	if err != nil {
		return nil, fmt.Errorf("Malformed ModuleSource: spec.registry: %w", err)
	}

	decodedDockerCfg := struct {
		Auths map[string]struct {
			Auth     string `json:"auth,omitempty"`
			User     string `json:"username,omitempty"`
			Password string `json:"password,omitempty"`
		} `json:"auths"`
	}{}
	if err := json.Unmarshal(buf, &decodedDockerCfg); err != nil {
		return nil, fmt.Errorf("Decode dockerCfg: %w", err)
	}

	if decodedDockerCfg.Auths == nil {
		return authn.Anonymous, nil
	}
	registryAuth, hasRegistryCreds := decodedDockerCfg.Auths[registryURL.Host]
	if !hasRegistryCreds {
		return authn.Anonymous, nil
	}

	if registryAuth.Auth != "" {
		return authn.FromConfig(authn.AuthConfig{
			Auth: registryAuth.Auth,
		}), nil
	}

	if registryAuth.User != "" && registryAuth.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: registryAuth.User,
			Password: registryAuth.Password,
		}), nil
	}

	return authn.Anonymous, nil
}

func PushModulesToRegistry(
	modulesDir string,
	registryPath string,
	authProvider authn.Authenticator,
	insecure, skipVerifyTLS bool,
) error {
	dirEntries, err := os.ReadDir(modulesDir)
	if err != nil {
		return fmt.Errorf("Read modules directory: %w", err)
	}

	refOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptions(authProvider, insecure, skipVerifyTLS)

	for i, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		moduleRegistryPath := path.Join(registryPath, moduleName)
		moduleReleasesRegistryPath := path.Join(registryPath, moduleName, "release")

		log.InfoF("Pushing module %s... [%d / %d]\n", moduleName, i+1, len(dirEntries))

		moduleLayout, err := layout.FromPath(filepath.Join(modulesDir, moduleName))
		if err != nil {
			return fmt.Errorf("Module %s: Read OCI layout: %w", moduleName, err)
		}
		moduleReleasesLayout, err := layout.FromPath(filepath.Join(modulesDir, moduleName, "release"))
		if err != nil {
			return fmt.Errorf("Module %s: Read OCI layout: %w", moduleName, err)
		}

		if err = pushLayoutToRepo(moduleLayout, moduleRegistryPath, authProvider, insecure, skipVerifyTLS); err != nil {
			return fmt.Errorf("Push module to registry: %w", err)
		}

		log.InfoF("Pushing releases for module %s...\n", moduleName)
		if err = pushLayoutToRepo(moduleReleasesLayout, moduleReleasesRegistryPath, authProvider, insecure, skipVerifyTLS); err != nil {
			return fmt.Errorf("Push module to registry: %w", err)
		}

		log.InfoF("Pushing index tag for module %s...\n", moduleName)

		imageRef, err := name.ParseReference(registryPath+":"+moduleName, refOpts...)
		if err != nil {
			return fmt.Errorf("Parse image reference: %w", err)
		}

		img, err := random.Image(16, 1)
		if err != nil {
			return fmt.Errorf("random.Image: %w", err)
		}

		if err = remote.Write(imageRef, img, remoteOpts...); err != nil {
			return fmt.Errorf("Write module index tag: %w", err)
		}

		log.InfoF("✅Module %s pushed successfully\n", moduleName)
	}

	return nil
}

func pushLayoutToRepo(
	imagesLayout layout.Path,
	registryRepo string,
	authProvider authn.Authenticator,
	insecure, skipVerifyTLS bool,
) error {
	refOpts, remoteOpts := mirror.MakeRemoteRegistryRequestOptions(authProvider, insecure, skipVerifyTLS)

	index, err := imagesLayout.ImageIndex()
	if err != nil {
		return fmt.Errorf("Read OCI Image Index: %w", err)
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return fmt.Errorf("Parse OCI Image Index Manifest: %w", err)
	}

	pushCount := 1
	for _, imageDesc := range indexManifest.Manifests {
		tag := imageDesc.Annotations["io.deckhouse.image.short_tag"]
		imageRef := registryRepo + ":" + tag

		log.InfoF("[%d / %d] Pushing image %s...\t", pushCount, len(indexManifest.Manifests), imageRef)
		img, err := index.Image(imageDesc.Digest)
		if err != nil {
			return fmt.Errorf("Read image: %w", err)
		}

		ref, err := name.ParseReference(imageRef, refOpts...)
		if err != nil {
			return fmt.Errorf("Parse image reference: %w", err)
		}
		if err = remote.Write(ref, img, remoteOpts...); err != nil {
			return fmt.Errorf("Write %s to registry: %w", ref.String(), err)
		}
		log.InfoLn("✅")
		pushCount += 1
	}

	return nil
}
