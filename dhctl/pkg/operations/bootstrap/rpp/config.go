// Copyright 2026 Flant JSC
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

package rpp

import (
	"strings"

	
	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

type ClientConfigGetter struct {
	registry.ClientConfig
}

func NewClientConfigGetter(config registry_config.Data) (*ClientConfigGetter) {
	return &ClientConfigGetter{
		ClientConfig: registry.ClientConfig{
			Repository: config.ImagesRepo,
			Scheme:     strings.ToLower(string(config.Scheme)),
			CA:         config.CA,
			Auth:       config.AuthBase64(),
		},
	}
}

func (r *ClientConfigGetter) Get(_ string) (*registry.ClientConfig, error) {
	return &r.ClientConfig, nil
}