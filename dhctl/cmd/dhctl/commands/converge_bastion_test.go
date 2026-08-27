// Copyright 2026 Flant JSC
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

package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// The converge cache is keyed by the kubeconfig's PATH, not its content
// (pkg/state/cache/init.go, GetCacheIdentityFromKubeconfig). Pointing the option
// at the temporary copy would give every run a new identity and lose the state
// an interrupted converge resumes from.
func TestTheKubeconfigOptionSurvivesTheBastionChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "admin.kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\n"), 0o600))

	opts := options.New()
	opts.Kube.Config = path

	// No SSH provider: the guards return the provider untouched, which is the
	// only branch reachable without a bastion to dial.
	_, stop, err := kubeProviderThroughBastion(t.Context(), opts, nil, nil)
	require.NoError(t, err)
	defer stop()

	require.Equal(t, path, opts.Kube.Config, "the cache identity is derived from this path")
}
