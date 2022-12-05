/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"
	"os"
	"path"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

/*
The purpose of this script is to mutate base images metadata before uploading them.

The real use case - avoid image labels inheritance. We do not want to have labels like `maintainer` or `version`
on our software. Deckhouse team is the one who gives guarantees.

USAGE:

1. Compile it using makefile - make bin/regcopy
2. Copy images by executing `regcopy alpine:3.16.0`
*/

const (
	BaseImagesRegistryPathEnv = "BASE_IMAGES_REGISTRY_PATH"
	DefaultRegistry           = "registry-write.deckhouse.io/base_images"
)

func mutateConfig(base v1.Image) (v1.Image, error) {
	file, err := base.ConfigFile()
	if err != nil {
		return nil, err
	}

	// Remove all labels <- the actual work
	// TODO(nabokihms): add informational labels in the future, e.g., maintainer, heritage
	file.Config.Labels = nil

	base, err = mutate.ConfigFile(base, file)
	if err != nil {
		return nil, err
	}

	return base, nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("no images provided to copy")
	}

	for _, imageToCopy := range os.Args[1:] {
		log.Printf("start copying image %q", imageToCopy)

		remoteRef, err := name.ParseReference(imageToCopy)
		if err != nil {
			log.Fatal(err)
		}

		baseRegistry := os.Getenv(BaseImagesRegistryPathEnv)
		if baseRegistry == "" {
			baseRegistry = DefaultRegistry
		}

		newRef, err := name.ParseReference(path.Join(baseRegistry, remoteRef.String()))
		if err != nil {
			log.Fatal(err)
		}

		remoteImage, err := remote.Image(remoteRef)
		if err != nil {
			log.Fatal(err)
		}

		digest, err := remoteImage.Digest()
		if err != nil {
			log.Fatal(err)
		}

		remoteImage, err = mutateConfig(remoteImage)
		if err != nil {
			log.Fatal(err)
		}

		if err := remote.Write(newRef, remoteImage, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
			log.Fatal(err)
		}

		resultImage, err := remote.Image(newRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			log.Fatal(err)
		}

		resultDigest, err := resultImage.Digest()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("sucessfully copied \"%s@%s\" -> \"%s@%s\"", remoteRef.String(), digest, newRef.String(), resultDigest)
	}
}
