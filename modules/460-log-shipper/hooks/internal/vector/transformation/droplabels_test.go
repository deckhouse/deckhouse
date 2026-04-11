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

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

func TestDropLabelsVRL(t *testing.T) {
	t.Run("delete paths", func(t *testing.T) {
		got, paths, err := DropLabelsVRL(v1alpha1.DropLabelsSpec{
			Labels: []string{".first", ".second"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{".first", ".second"}, paths)
		assert.Equal(t, "if exists(.first) {\n  del(.first)\n}\n"+
			"if exists(.second) {\n  del(.second)\n}", got)
	})

	t.Run("keepChildKeys", func(t *testing.T) {
		got, paths, err := DropLabelsVRL(v1alpha1.DropLabelsSpec{
			Labels:        []string{".pod_labels"},
			KeepChildKeys: []string{"app", "group"},
		})
		require.NoError(t, err)
		assert.Nil(t, paths)
		assert.Equal(t, `obj, err = get(., ["pod_labels"])
if err == null && is_object(obj) {
  filtered = {}
  v, err2 = get(obj, ["app"])
  if err2 == null {
    filtered = set!(filtered, ["app"], v)
  }
  v, err2 = get(obj, ["group"])
  if err2 == null {
    filtered = set!(filtered, ["group"], v)
  }
  . = set!(., ["pod_labels"], filtered)
}`, got)
	})
}
