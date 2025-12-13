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
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

func TestCleanupsDoesNotPanic(t *testing.T) {
	sshProvider := sshclient.NewDefaultSSHProviderWithFunc(func() (node.SSHClient, error) {
		return gossh.NewClientFromFlags(context.Background())
	})

	provider := newKubeClientProvider(sshProvider)

	cleanupTest := func() {
		provider.Cleanup(true)
	}
	require.NotPanics(t, cleanupTest)
	// double call not panic
	require.NotPanics(t, cleanupTest)
}
