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
	"strings"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// Validate runs the four cloud checks over the snapshot. It performs no I/O.
//
// Its Error is a statement about the NodeGroup, not about this pass: retrying changes nothing
// until a human fixes the NodeGroup. That is why it travels beside the reconcile error rather than
// as one — the status controller publishes it, the bashible context falls back to its last good
// entry, and the MachineDeployment reconciler skips rendering.
func Validate(ng *v1.NodeGroup, snap Snapshot) CloudCheckResult {
	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral || snap.Provider.InstanceClassKind == "" {
		return CloudCheckResult{Processed: false}
	}

	// The provider names a kind but no version to read it at. Reporting it as a validation
	// error is what every consumer already handles: rendering is skipped, and the bashible
	// context keeps the entry it published last instead of dropping the cloud fields.
	if snap.Provider.InstanceClassAPIVersion == "" {
		return CloudCheckResult{Error: fmt.Sprintf(
			"Cloud provider has not published %s yet. The %s cannot be read until it does.",
			nodecommon.InstanceClassAPIVersionKey, snap.Provider.InstanceClassKind)}
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

	if result := RunCloudChecks(in); !result.Processed {
		return result
	}
	// Provider- and field-scoped checks. RunCloudChecks stayed generic; the ones below reject a
	// specific field on a specific provider, and both need snap.InstanceClass, which is only
	// populated after check #1 and check #2 above pass (see BuildSnapshot).
	if err := openstackTagsError(ng, snap); err != nil {
		return CloudCheckResult{Error: err.Error()}
	}
	return CloudCheckResult{Processed: true}
}

// openstackTagsError enforces the two contracts that OpenStackInstanceClass.spec.tags relies on
// but that neither the CRD schema nor the render templates can enforce alone:
//
//   - the tag is only rendered by the CAPI provider template (capi/template.yaml), so an MCM
//     NodeGroup would silently drop it — better to reject up-front than to leave the operator
//     wondering why their preemptible workers never became preemptible;
//   - even under CAPI, the tag is only known to trigger preemption on Selectel. On other OpenStack
//     providers the tag is attached to the VM but ignored, which is the exact silent failure the
//     field exists to prevent.
//
// A missing authURL (partial snapshot during bootstrap) is treated as "not yet Selectel", not "not
// Selectel" — the reconcile retries once the discovery hook publishes the connection block.
func openstackTagsError(ng *v1.NodeGroup, snap Snapshot) error {
	if ng.Spec.CloudInstances == nil || ng.Spec.CloudInstances.ClassReference.Kind != "OpenStackInstanceClass" {
		return nil
	}
	tags, ok := snap.InstanceClass["tags"].([]any)
	if !ok || len(tags) == 0 {
		return nil
	}
	className := ng.Spec.CloudInstances.ClassReference.Name

	if ComputeEngine(ng, snap.Provider) != engineCAPI {
		return fmt.Errorf(
			"OpenStackInstanceClass '%s' sets spec.tags, but this NodeGroup does not run on the CAPI engine (raw Nova tags are only rendered by the CAPI provider template). "+
				"Either migrate the NodeGroup to the CAPI engine, or remove spec.tags from the OpenStackInstanceClass. "+
				"Note that the engine is chosen per NodeGroup — a single InstanceClass shared between a CAPI NodeGroup and a non-CAPI one will silently break the non-CAPI one.",
			className)
	}

	authURL := authURLOf(snap.Provider)
	if authURL == "" {
		return nil
	}
	lc := strings.ToLower(authURL)
	if strings.Contains(lc, "selcloud.ru") || strings.Contains(lc, "selectel") {
		return nil
	}
	return fmt.Errorf(
		"OpenStackInstanceClass '%s' sets spec.tags, but this cluster's connection.authURL (%q) does not look like Selectel (selcloud.ru / selectel). "+
			"Raw Nova tags are a Selectel-specific preemption mechanism; on other OpenStack providers the tag will be attached to the VM but ignored. "+
			"Remove spec.tags, or use the field only on Selectel-hosted clusters where it is known to work.",
		className, authURL)
}

// authURLOf reads connection.authURL from the provider's registration. The provider tree lives in
// CloudVariables (see CloudProviderRegistration), and connection is a nested map inside it — so a
// missing key returns "" rather than panicking on the type assertion, matching how RunCloudChecks
// treats a partial snapshot elsewhere.
func authURLOf(reg CloudProviderRegistration) string {
	connection, ok := reg.CloudVariables["connection"].(map[string]any)
	if !ok {
		return ""
	}
	authURL, _ := connection["authURL"].(string)
	return authURL
}
