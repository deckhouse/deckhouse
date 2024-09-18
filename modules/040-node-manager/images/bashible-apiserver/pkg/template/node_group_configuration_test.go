/*
Copyright 2024 Flant JSC

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
