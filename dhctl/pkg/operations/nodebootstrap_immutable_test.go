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

package operations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// Only an immutable group is configured by the installer. Every other group is
// created the way it always was, and taking the new path for one of them would
// ask its layout for an address no provider publishes.
func TestOnlyAnImmutableGroupIsConfiguredByTheInstaller(t *testing.T) {
	t.Parallel()

	configure := func(context.Context, *client.KubernetesClient, string, string, string) error { return nil }

	cases := map[string]struct {
		group     config.TerraNodeGroupSpec
		configure ConfigureImmutableNode
		want      bool
	}{
		"an immutable group": {
			group:     config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"},
			configure: configure,
			want:      true,
		},
		"a group with no system type": {
			group:     config.TerraNodeGroupSpec{Name: "worker"},
			configure: configure,
			want:      false,
		},
		"a group of another system type": {
			group:     config.TerraNodeGroupSpec{Name: "worker", SystemType: "Classic"},
			configure: configure,
			want:      false,
		},
		"an immutable group on a bootstrap that configures nothing": {
			group:     config.TerraNodeGroupSpec{Name: "front", SystemType: "Immutable"},
			configure: nil,
			want:      false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, configureFor(tc.group, tc.configure) != nil)
		})
	}
}

// A group the installer configures is created bare: its machines take a document
// pushed to them, and the cloud config published for the group is a bashible
// script an immutable machine cannot run.
func TestAConfiguredGroupIsCreatedWithNoCloudConfig(t *testing.T) {
	t.Parallel()

	configure := func(context.Context, *client.KubernetesClient, string, string, string) error { return nil }

	cloudConfig, err := groupCloudConfig(t.Context(), nil, "front", configure)
	require.NoError(t, err, "a configured group must not ask the cluster for a cloud config")
	require.Empty(t, cloudConfig)
}
