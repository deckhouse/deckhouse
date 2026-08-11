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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
)

// dhctl-server writes no default kubeconfig and its bootstrap response carries
// none, so a Commander-mode run without --kubeconfig-out would end with no way
// into the cluster. Caught before anything is created rather than after.
func TestImmutableKubeconfigOut(t *testing.T) {
	tests := []struct {
		name          string
		commanderMode bool
		kubeconfigOut string
		wantErr       bool
	}{
		{name: "the CLI writes a default path", commanderMode: false},
		{name: "the CLI with an explicit path", kubeconfigOut: "/tmp/admin.kubeconfig"},
		{name: "Commander with a path", commanderMode: true, kubeconfigOut: "/tmp/admin.kubeconfig"},
		{name: "Commander without a path", commanderMode: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := ImmutableKubeconfigOut(
				&options.BootstrapOptions{KubeconfigOut: tt.kubeconfigOut},
				tt.commanderMode,
			)

			err := check.Run(t.Context())
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, immutable.ErrKubeconfigOutRequired)
			require.Contains(t, err.Error(), "--kubeconfig-out")
		})
	}
}
