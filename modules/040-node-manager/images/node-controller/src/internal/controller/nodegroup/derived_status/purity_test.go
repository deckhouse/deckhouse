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

package derived_status

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// The core must stay free of I/O. A client in derive.go or validate.go means a read crept back into
// the pure half — which is how the double reads and the swallowed errors got there in the first
// place: every reader that lived beside the logic was free to answer "absent" on failure.
//
// derive.go may hold a context: it logs. It must not hold a client.
func TestCoreFilesHaveNoClient(t *testing.T) {
	forbidden := []string{"s.Client", "client.Client", "client.Reader", "*Service"}

	for _, file := range []string{"derive.go", "validate.go"} {
		t.Run(file, func(t *testing.T) {
			content, err := os.ReadFile(file)
			require.NoError(t, err)

			for _, token := range forbidden {
				require.NotContains(t, string(content), token,
					"%s must stay pure; move the read into snapshot.go", file)
			}
		})
	}
}

// Purity is worth a behavioural check too: the same inputs must give the same output, so the only
// thing that may vary between two calls is the injected clock.
func TestDerive_IsDeterministic(t *testing.T) {
	restore := epochTimestampAccessor
	epochTimestampAccessor = func() int64 { return 1_700_000_000 }
	t.Cleanup(func() { epochTimestampAccessor = restore })

	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	snap := Snapshot{
		Provider:    CloudProviderRegistration{Type: "aws", MachineClassKind: "AWSMachineClass"},
		ClusterUUID: "uuid-1",
	}

	first, err := Derive(t.Context(), ng, snap)
	require.NoError(t, err)
	second, err := Derive(t.Context(), ng, snap)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, engineMCM, first.Engine)
	require.NotEmpty(t, first.UpdateEpoch)
}

// Validate is pure in the same sense, and its verdict must not depend on call order.
func TestValidate_IsDeterministic(t *testing.T) {
	ng := &v1.NodeGroup{}
	ng.Name = "worker"
	ng.Spec.NodeType = v1.NodeTypeCloudEphemeral
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}
	snap := Snapshot{
		Provider: CloudProviderRegistration{
			InstanceClassKind:       "AWSInstanceClass",
			InstanceClassAPIVersion: "v1",
		},
		KnownClassNames: []string{"worker"},
	}

	require.Equal(t, Validate(ng, snap), Validate(ng, snap))
	require.True(t, Validate(ng, snap).Processed)
}
