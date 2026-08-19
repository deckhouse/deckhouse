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

package checks

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// The admin kubeconfig is collected once and is the only way into the cluster,
// so both ways of losing it are caught before anything is created: a Commander
// run that names no path, and a path the tmp cleaner sweeps at exit.
func TestImmutableKubeconfigOut(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		commanderMode bool
		kubeconfigOut string
		wantErr       error
		wantMessage   string
	}{
		{name: "the CLI writes a default path", commanderMode: false},
		{name: "the CLI with an explicit path", kubeconfigOut: filepath.Join(t.TempDir(), "admin.kubeconfig")},
		{name: "Commander with a path", commanderMode: true, kubeconfigOut: filepath.Join(t.TempDir(), "admin.kubeconfig")},
		{
			name:          "Commander without a path",
			commanderMode: true,
			wantErr:       immutable.ErrKubeconfigOutRequired,
			wantMessage:   "--kubeconfig-out",
		},
		{
			name:          "a path dhctl empties on its way out",
			kubeconfigOut: filepath.Join(tmpDir, "admin.yaml"),
			wantMessage:   "which dhctl empties when it exits",
		},
		{
			// The tmp cleaner spares this suffix, so the path survives.
			name:          "a path inside it that the cleaner spares",
			kubeconfigOut: filepath.Join(tmpDir, cache.AdminKubeconfigName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := ImmutableKubeconfigOut(
				&options.BootstrapOptions{KubeconfigOut: tt.kubeconfigOut},
				&options.GlobalOptions{TmpDir: tmpDir},
				tt.commanderMode,
			)

			err := check.Run(t.Context())
			if tt.wantErr == nil && tt.wantMessage == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
			require.Contains(t, err.Error(), tt.wantMessage)
		})
	}
}
