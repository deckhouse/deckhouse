/*
Copyright 2026 Flant JSC

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

package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestIsLeftoverWrap(t *testing.T) {
	t.Run("nil project", func(t *testing.T) {
		assert.False(t, IsLeftoverWrap(nil))
	})
	t.Run("handmade empty template", func(t *testing.T) {
		assert.False(t, IsLeftoverWrap(project("foo", nil, "")))
	})
	t.Run("wrap label", func(t *testing.T) {
		assert.True(t, IsLeftoverWrap(project("foo", map[string]string{
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		}, "")))
	})
}
