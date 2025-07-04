/*
Copyright 2025 Flant JSC

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
