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
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// liveInstanceClassChecksum is the value a running cluster carries on the MachineDeployment it
// renders for this NodeGroup:
//
//	kubectl -n d8-cloud-instance-manager get machinedeployments.cluster.x-k8s.io \
//	  zykov-dev-u2-worker-0f7c2f04 -o jsonpath='{.metadata.annotations.checksum/instance-class}'
//
// It names an immutable machine template, so reproducing it byte for byte is the strongest
// statement this package can make: the rewrite publishes an element that hashes to what the
// deployed controller already hashed, and no machine is recreated by the change.
//
// The DVP template reads only .nodeGroup.instanceClass, so no cluster credentials are involved.
const liveInstanceClassChecksum = "3040a219bf773e7f8d8926575bbb4beb339c7c4ca000758d39a7d0d1be629172"

func TestResolvedNodeGroup_ReproducesLiveInstanceClassChecksum(t *testing.T) {
	template, err := os.ReadFile("../../../machinetemplate/testdata/v1/dvp/instance-class.checksum")
	require.NoError(t, err, "DVP instance-class checksum template")

	var rawSpec map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(liveWorkerSpecJSON), &rawSpec))

	in := ResolveInput{
		Name:           "worker",
		NodeType:       v1.NodeTypeCloudEphemeral,
		RawSpec:        rawSpec,
		CloudProcessed: true,
	}
	result := Result{
		Engine:        "CAPI",
		CRIType:       "Containerd",
		Zones:         []string{"default"},
		InstanceClass: rawExtension(liveWorkerInstanceClassJSON),
	}

	nodeGroupValues := ResolveNodeGroup(in, result).ToMap()

	got, err := machineclass.RenderChecksum(template, nodeGroupValues, map[string]interface{}{})
	require.NoError(t, err)
	require.Equal(t, liveInstanceClassChecksum, got,
		"the published element must hash to what the deployed controller hashed; a mismatch renames "+
			"the machine template and recreates every VM in the NodeGroup")
}
