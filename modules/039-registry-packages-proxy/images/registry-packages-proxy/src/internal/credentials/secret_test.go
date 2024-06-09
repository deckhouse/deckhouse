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

package credentials

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToClientConfig(t *testing.T) {
	t.Run("Path with leading slash", func(t *testing.T) {
		sd := registrySecretData{
			Address: "registry.deckhouse.io",
			Path:    "/deckhouse/ee",
		}
		c, err := sd.toClientConfig()
		require.NoError(t, err)
		require.Equal(t, c.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
	t.Run("Path without leading slash", func(t *testing.T) {
		sd := registrySecretData{
			Address: "registry.deckhouse.io",
			Path:    "deckhouse/ee",
		}
		c, err := sd.toClientConfig()
		require.NoError(t, err)
		require.Equal(t, c.Repository, "registry.deckhouse.io/deckhouse/ee")
	})
}
