package types

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

	//assert.True(t, m.Requirements.Bootstrapped)
	assert.Equal(t, "testxxz", m.Name)
	assert.Equal(t, uint32(340), m.Weight)
	assert.Equal(t, "true", m.Requirements.Bootstrapped)
	assert.Equal(t, ">= 1.31", m.Requirements.Kubernetes)
	assert.Equal(t, ">= 1.67", m.Requirements.Deckhouse)
	assert.Equal(t, ">= 0.0.0", m.Requirements.ParentModules["prometheus"])
}
