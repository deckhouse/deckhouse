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

package transformation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha2"
)

func TestReplaceKeysVRL(t *testing.T) {
	t.Run("two labels", func(t *testing.T) {
		got, err := ReplaceKeysVRL(v1alpha2.ReplaceKeysSpec{
			Source: ".",
			Target: "_",
			Labels: []string{".pod_labels", ".examples"},
		})
		require.NoError(t, err)
		assert.Equal(t, `if exists(.pod_labels) {
  .pod_labels = map_keys(
    object!(.pod_labels), recursive: true
  ) -> |key| {
    replace(key, ".", "_")
  }
}
if exists(.examples) {
  .examples = map_keys(
    object!(.examples), recursive: true
  ) -> |key| {
    replace(key, ".", "_")
  }
}`, got)
	})
}
