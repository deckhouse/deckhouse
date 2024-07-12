// Copyright 2024 Flant JSC
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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type TagsResolver struct {
	tagsDigestsMapping map[string]v1.Hash
}

func NewTagsResolver() *TagsResolver {
	return &TagsResolver{tagsDigestsMapping: make(map[string]v1.Hash)}
}

func NopTagToDigestMappingFunc(_ string) *v1.Hash {
	return nil
}

func (r *TagsResolver) ResolveTagsDigestsForImageLayouts(mirrorCtx *Context, layouts *ImageLayouts) error {
	imageSets := []map[string]struct{}{
		layouts.DeckhouseImages,
		layouts.ReleaseChannelImages,
		layouts.InstallImages,
	}

	for _, moduleImageLayout := range layouts.Modules {
		imageSets = append(imageSets, moduleImageLayout.ModuleImages)
		imageSets = append(imageSets, moduleImageLayout.ReleaseImages)
	}

	for _, imageSet := range imageSets {
		if err := r.ResolveTagsDigestsFromImageSet(
			imageSet,
			mirrorCtx.RegistryAuth,
			mirrorCtx.Insecure,
			mirrorCtx.SkipTLSVerification,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *TagsResolver) ResolveTagsDigestsFromImageSet(
	imageSet map[string]struct{},
	authProvider authn.Authenticator,
	insecure, skipTLSVerification bool,
) error {
	nameOpts, remoteOpts := MakeRemoteRegistryRequestOptions(authProvider, insecure, skipTLSVerification)
	for imageRef := range imageSet {
		if digestRegex.MatchString(imageRef) {
			continue
		}

		ref, err := name.ParseReference(imageRef, nameOpts...)
		if err != nil {
			return fmt.Errorf("parse %q image reference: %w", imageRef, err)
		}
		desc, err := remote.Head(ref, remoteOpts...)
		if err != nil {
			if isImageNotFoundError(err) {
				continue
			}

			return fmt.Errorf("get image descriptor for %q: %w", imageRef, err)
		}

		r.tagsDigestsMapping[imageRef] = desc.Digest
	}

	return nil
}

func (r *TagsResolver) GetTagDigest(imageRef string) *v1.Hash {
	digest, found := r.tagsDigestsMapping[imageRef]
	if !found {
		return nil
	}
	return &digest
}
