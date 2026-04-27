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
	"testing"

	"github.com/stretchr/testify/require"

	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

func TestNewRegistryClientConfigGetter(t *testing.T) {
	t.Run("Path with leading slash", func(t *testing.T) {
		config := registry_config.Data{
			ImagesRepo: "registry.deckhouse.io/deckhouse/ee",
			Username:   "",
			Password:   "",
		}
		getter := NewClientConfigGetter(config)
		require.Equal(t, getter.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Path without leading slash", func(t *testing.T) {
		config := registry_config.Data{
			ImagesRepo: "registry.deckhouse.io/deckhouse/ee",
			Username:   "",
			Password:   "",
		}
		getter := NewClientConfigGetter(config)
		require.Equal(t, getter.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Host with port, path with leading slash", func(t *testing.T) {
		config := registry_config.Data{
			ImagesRepo: "registry.deckhouse.io:30000/deckhouse/ee",
			Username:   "",
			Password:   "",
		}
		getter := NewClientConfigGetter(config)
		require.Equal(t, getter.Repository, "registry.deckhouse.io:30000/deckhouse/ee")
	})
	t.Run("Host with port, path without leading slash", func(t *testing.T) {
		config := registry_config.Data{
			ImagesRepo: "registry.deckhouse.io:30000/deckhouse/ee",
			Username:   "",
			Password:   "",
		}
		getter := NewClientConfigGetter(config)
		require.Equal(t, getter.Repository, "registry.deckhouse.io:30000/deckhouse/ee")
	})
}
