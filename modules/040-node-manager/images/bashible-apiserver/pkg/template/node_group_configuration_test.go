package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsEqual(t *testing.T) {
	ngc1 := NodeGroupConfigurationSpec{
		Content:    "test",
		Weight:     100,
		NodeGroups: []string{"*"},
		Bundles:    []string{"*"},
	}

	ngc2 := NodeGroupConfigurationSpec{
		Content:    "test",
		Weight:     100,
		NodeGroups: []string{"*"},
		Bundles:    []string{"*"},
	}

	t.Run("Test equality", func(t *testing.T) {
		res := ngc1.IsEqual(ngc2)
		require.True(t, res)
	})
}
