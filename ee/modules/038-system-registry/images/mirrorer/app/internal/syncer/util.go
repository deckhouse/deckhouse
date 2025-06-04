/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package syncer

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func getImageConfigFile(manifest *remote.Descriptor) (*v1.ConfigFile, error) {
	image, err := manifest.Image()
	if err != nil {
		return nil, fmt.Errorf("cannot get image: %w", err)
	}

	cfg, err := image.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("cannot get config file: %w", err)
	}

	return cfg, nil
}
