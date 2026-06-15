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

package v1alpha3

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNamespaceStatusUnmarshal_BackwardCompat guards against the startup crash caused by projects
// whose status.namespaces was stored by older controllers as a list of plain strings.
func TestNamespaceStatusUnmarshal_BackwardCompat(t *testing.T) {
	// legacy form: list of plain namespace-name strings
	var legacy ProjectStatus
	err := json.Unmarshal([]byte(`{"namespaces":["foo","foo-bar"]}`), &legacy)
	assert.NoError(t, err)
	assert.Equal(t, []NamespaceStatus{{Name: "foo"}, {Name: "foo-bar"}}, legacy.Namespaces)

	// current form: list of objects
	var current ProjectStatus
	err = json.Unmarshal([]byte(`{"namespaces":[{"name":"foo","kind":"Main"},{"name":"foo-bar","kind":"Additional"}]}`), &current)
	assert.NoError(t, err)
	assert.Equal(t, []NamespaceStatus{{Name: "foo", Kind: "Main"}, {Name: "foo-bar", Kind: "Additional"}}, current.Namespaces)

	// round-trip: re-marshaling always produces the object form
	out, err := json.Marshal(current.Namespaces)
	assert.NoError(t, err)
	assert.JSONEq(t, `[{"name":"foo","kind":"Main"},{"name":"foo-bar","kind":"Additional"}]`, string(out))
}
