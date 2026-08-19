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
	"fmt"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
)

// Validate runs the four cloud checks over the snapshot. It performs no I/O.
//
// Its Error is a statement about the NodeGroup, not about this pass: retrying changes nothing
// until a human fixes the NodeGroup. That is why it travels beside the reconcile error rather than
// as one — the status controller publishes it, the bashible context falls back to its last good
// entry, and the MachineDeployment reconciler skips rendering.
func Validate(ng *v1.NodeGroup, snap Snapshot) CloudCheckResult {
	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return CloudCheckResult{Processed: false}
	}

	// The provider names a kind but no version to read it at. Reporting it as a validation
	// error is what every consumer already handles: rendering is skipped, and the bashible
	// context keeps the entry it published last instead of dropping the cloud fields.
	//
	// A provider that published no kind at all is left to RunCloudChecks, which reports the
	// NodeGroup unprocessed rather than wrong.
	if snap.Provider.InstanceClassKind != "" && snap.Provider.InstanceClassAPIVersion == "" {
		return CloudCheckResult{Error: fmt.Sprintf(
			"Cloud provider has not published %s yet. The %s cannot be read until it does.",
			cloudprovider.InstanceClassAPIVersionKey, snap.Provider.InstanceClassKind)}
	}

	in := CloudCheckInput{
		NodeType:        ng.Spec.NodeType,
		KindInUse:       snap.Provider.InstanceClassKind,
		KnownClassNames: snap.KnownClassNames,
		DefaultZones:    snap.DefaultZones,
		CapacityErr:     snap.CapacityErr,
	}
	if ng.Spec.CloudInstances != nil {
		in.ClassRefKind = ng.Spec.CloudInstances.ClassReference.Kind
		in.ClassRefName = ng.Spec.CloudInstances.ClassReference.Name
		in.MinPerZone = ng.Spec.CloudInstances.MinPerZone
		in.MaxPerZone = ng.Spec.CloudInstances.MaxPerZone
		in.SpecZones = ng.Spec.CloudInstances.Zones
	}

	return RunCloudChecks(in)
}
