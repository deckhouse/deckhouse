/*
Copyright 2025 Flant JSC

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
package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_Process(t *testing.T) {
	t.Run("Generate HTTP secret if empty", func(t *testing.T) {
		state := &State{HTTP: ""}
		err := state.Process()
		require.NoError(t, err)
		assert.NotEmpty(t, state.HTTP, "HTTP secret should be generated")
	})

	t.Run("Keep existing HTTP secret", func(t *testing.T) {
		existingSecret := "my-super-secret-value"
		state := &State{HTTP: existingSecret}
		err := state.Process()
		require.NoError(t, err)
		assert.Equal(t, existingSecret, state.HTTP, "Existing HTTP secret should be preserved")
	})

	t.Run("Handle whitespace secret", func(t *testing.T) {
		state := &State{HTTP: "   "}
		err := state.Process()
		require.NoError(t, err)
		assert.NotEmpty(t, state.HTTP)
		assert.NotEqual(t, "   ", state.HTTP, "Whitespace secret should be replaced")
	})
}
