// Copyright 2025 Flant JSC
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

package types //nolint: revive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshal(t *testing.T) {
	data := `
name: testxxz
weight: 340

requirements:
  bootstrapped: true
  deckhouse: ">= 1.67"
  kubernetes: ">= 1.31"
  modules:
    prometheus: ">= 0.0.0"
    control-plane-manager: ">= 0.0.0"
`

	var m Definition

	err := yaml.Unmarshal([]byte(data), &m)
	require.NoErrorf(t, err, "try unmarshal yaml\n%s", data)

	// assert.True(t, m.Requirements.Bootstrapped)
	assert.Equal(t, "testxxz", m.Name)
	assert.Equal(t, uint32(340), m.Weight)
	assert.Equal(t, "true", m.Requirements.Bootstrapped)
	assert.Equal(t, ">= 1.31", m.Requirements.Kubernetes)
	assert.Equal(t, ">= 1.67", m.Requirements.Deckhouse)
	assert.Equal(t, ">= 0.0.0", m.Requirements.ParentModules["prometheus"])
}
