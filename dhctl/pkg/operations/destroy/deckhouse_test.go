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

package destroy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

func TestCleanupsDoesNotPanic(t *testing.T) {
	sshProvider := func() (node.SSHClient, error) {
		return gossh.NewClientFromFlags()
	}

	state := NewDestroyState(&cache.DummyCache{})

	destroyer := NewDeckhouseDestroyer(sshProvider, state, DeckhouseDestroyerOptions{CommanderMode: false})

	cleanupTest := func() {
		destroyer.Cleanup(true)
	}
	require.NotPanics(t, cleanupTest)
	// double call not panic
	require.NotPanics(t, cleanupTest)
}
