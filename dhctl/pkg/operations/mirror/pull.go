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
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func PullInstallers(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull installers")
	if err := PullImageSet(
		mirrorCtx.RegistryAuth,
		layouts.Install,
		layouts.InstallImages,
		mirrorCtx.Insecure,
		mirrorCtx.SkipTLSVerification,
		false,
	); err != nil {
		return err
	}
	log.InfoLn("✅ All required installers are pulled!")
	return nil
}

func PullDeckhouseReleaseChannels(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse release channels information")
	if err := PullImageSet(
		mirrorCtx.RegistryAuth,
		layouts.ReleaseChannel,
		layouts.ReleaseChannelImages,
		mirrorCtx.Insecure,
		mirrorCtx.SkipTLSVerification,
		mirrorCtx.SpecificVersion != nil,
	); err != nil {
		return err
	}
	log.InfoLn("✅ Deckhouse release channels are pulled!")
	return nil
}

func PullDeckhouseImages(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse, this may take a while")
	if err := PullImageSet(
		mirrorCtx.RegistryAuth,
		layouts.Deckhouse,
		layouts.DeckhouseImages,
		mirrorCtx.Insecure,
		mirrorCtx.SkipTLSVerification,
		false,
	); err != nil {
		return err
	}
	log.InfoLn("✅ All required Deckhouse images are pulled!")
	return nil
}

func PullImageSet(
	authProvider authn.Authenticator,
	targetLayout layout.Path,
	imageSet map[string]struct{},
	insecure, skipVerifyTLS, allowMissingTags bool,
) error {
	pullCount := 1
	totalCount := len(imageSet)
	for imageTag := range imageSet {
		pullOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure, skipVerifyTLS)
		ref, err := name.ParseReference(imageTag, pullOpts...)
		if err != nil {
			return fmt.Errorf("parse image reference %q: %w", imageTag, err)
		}

		err = retry.NewLoop(
			fmt.Sprintf("[%d / %d] Pulling %s...", pullCount, totalCount, imageTag),
			6,
			10*time.Second,
		).Run(func() error {
			img, err := remote.Image(ref, remoteOpts...)
			if err != nil {
				if isImageNotFoundError(err) && allowMissingTags {
					log.WarnLn("⚠️ Not found in registry, skipping pull")
					return nil
				}

				return fmt.Errorf("pull image metadata: %w", err)
			}

			err = targetLayout.AppendImage(img,
				layout.WithPlatform(v1.Platform{Architecture: "amd64", OS: "linux"}),
				layout.WithAnnotations(map[string]string{
					"org.opencontainers.image.ref.name": imageTag,
					"io.deckhouse.image.short_tag":      imageTag[strings.LastIndex(imageTag, ":")+1:],
				}),
			)
			if err != nil {
				return fmt.Errorf("write image to index: %w", err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("pull image %q: %w", imageTag, err)
		}
		pullCount++
	}
	return nil
}

func PullModules(mirrorCtx *Context, layouts *ImageLayouts) error {
	log.InfoLn("Beginning to pull Deckhouse modules")
	for moduleName, moduleData := range layouts.Modules {
		if err := PullImageSet(mirrorCtx.RegistryAuth, moduleData.ModuleLayout, moduleData.ModuleImages, mirrorCtx.Insecure, mirrorCtx.SkipTLSVerification, false); err != nil {
			return fmt.Errorf("pull %q module: %w", moduleName, err)
		}
		if err := PullImageSet(mirrorCtx.RegistryAuth, moduleData.ReleasesLayout, moduleData.ReleaseImages, mirrorCtx.Insecure, mirrorCtx.SkipTLSVerification, true); err != nil {
			return fmt.Errorf("pull %q module release information: %w", moduleName, err)
		}
	}
	log.InfoLn("✅ Deckhouse modules pulled!")
	return nil
}
