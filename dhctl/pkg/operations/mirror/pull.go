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
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func PullInstallers(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull installers")
	if err := pullImageSet(mirrorCtx.RegistryAuth, layouts.Install, layouts.InstallImages, mirrorCtx.Insecure); err != nil {
		return err
	}
	log.InfoLn("✅ All required installers are pulled!")
	return nil
}

func PullDeckhouseReleaseChannels(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse release channels information")
	if err := pullImageSet(mirrorCtx.RegistryAuth, layouts.ReleaseChannel, layouts.ReleaseChannelImages, mirrorCtx.Insecure); err != nil {
		return err
	}
	log.InfoLn("✅ Deckhouse release channels are pulled!")
	return nil
}

func PullDeckhouseImages(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse, this may take a while")
	if err := pullImageSet(mirrorCtx.RegistryAuth, layouts.Deckhouse, layouts.DeckhouseImages, mirrorCtx.Insecure); err != nil {
		return err
	}
	log.InfoLn("✅ All required Deckhouse images are pulled!")
	return nil
}

func pullImageSet(
	authProvider authn.Authenticator,
	targetLayout layout.Path,
	imageSet map[string]struct{},
	insecure bool,
) error {
	pullCount := 1
	totalCount := len(imageSet)
	for imageTag := range imageSet {
		log.InfoF("[%d / %d] Pulling %s...\t", pullCount, totalCount, imageTag)

		pullOpts := []name.Option{}
		remoteOpts := []remote.Option{}
		if insecure {
			pullOpts = append(pullOpts, name.Insecure)
		}
		if authProvider != nil {
			remoteOpts = append(remoteOpts, remote.WithAuth(authProvider))
		}

		ref, err := name.ParseReference(imageTag, pullOpts...)
		if err != nil {
			return fmt.Errorf("parse image reference %q: %w", imageTag, err)
		}
		img, err := remote.Image(ref, remoteOpts...)
		if err != nil {
			return fmt.Errorf("pull image %q: %w", imageTag, err)
		}

		err = targetLayout.AppendImage(img,
			layout.WithPlatform(v1.Platform{Architecture: "amd64", OS: "linux"}),
			layout.WithAnnotations(map[string]string{
				"org.opencontainers.image.ref.name": imageTag,
				"io.deckhouse.image.short_tag":      imageTag[strings.LastIndex(imageTag, ":")+1:],
			}),
		)
		if err != nil {
			return fmt.Errorf("pull image %q: %w", imageTag, err)
		}
		log.InfoLn("✅")
		pullCount++
	}
	return nil
}

func PullModules(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse modules")
	for moduleName, moduleData := range layouts.Modules {
		if err := pullImageSet(mirrorCtx.RegistryAuth, moduleData.ModuleLayout, moduleData.ModuleImages, mirrorCtx.Insecure); err != nil {
			return fmt.Errorf("pull %q module: %w", moduleName, err)
		}
		if err := pullImageSet(mirrorCtx.RegistryAuth, moduleData.ReleasesLayout, moduleData.ReleaseImages, mirrorCtx.Insecure); err != nil {
			return fmt.Errorf("pull %q module release information: %w", moduleName, err)
		}
	}
	log.InfoLn("✅ Deckhouse modules pulled!")
	return nil
}
