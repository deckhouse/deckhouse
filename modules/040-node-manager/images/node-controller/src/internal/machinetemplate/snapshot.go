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

package machinetemplate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The annotations node-controller stamps on every generation object. They are part of the
// contract — the spec documents them — so their names and their encoding live here, next to the
// comparison that reads them, rather than in the controller.
const (
	AppliedInstanceClassAnnotation  = "node.deckhouse.io/applied-instance-class"
	AppliedProviderConfigAnnotation = "node.deckhouse.io/applied-provider-config"
	AppliedRolloutIDAnnotation      = "node.deckhouse.io/applied-rollout-id"
	AppliedGenerationAnnotation     = "node.deckhouse.io/applied-generation"
)

// Snapshot is what a generation object was built from.
//
// InstanceClass holds the WHOLE spec, not just the rolloutFields: the snapshot is the facts and
// rolloutFields is the policy, and keeping them apart is what lets a provider add or drop a
// rolloutField in a release without rolling anybody's machines.
type Snapshot struct {
	InstanceClass map[string]any
	// Provider holds ONLY the declared providerRolloutFields — the one place the facts/policy split
	// above is deliberately broken. The provider subtree of the cloud-provider Secret carries the
	// cloud credentials (vcd's password and apiToken, yandex's serviceAccountJSON, huaweicloud's
	// accessKey, openstack's connection.password), and this snapshot lives on a MachineTemplate that
	// is read far more widely than that Secret. See PickFields for what it costs.
	Provider  map[string]any
	RolloutID string
	// Generation is the counter in the object's name. Zero means the object predates v2 (it was
	// named by the v1 checksum and adopted), so the next generation created after it is 1.
	Generation int
}

// PickFields returns the subset of spec named by fields, and is how the provider config is reduced
// before it is written to a generation object.
//
// The cost of reducing it: widening providerRolloutFields in a provider release can roll machines,
// because a field the old snapshot never recorded compares as absent against its current value.
// That is the opposite of how rolloutFields behaves, and the contract doc says so — check the
// current value before adding an entry, or pair it with a release note.
func PickFields(spec map[string]any, fields []string) (map[string]any, error) {
	out := map[string]any{}
	for _, field := range fields {
		path := strings.Split(field, ".")
		value, found, err := unstructured.NestedFieldNoCopy(spec, path...)
		if err != nil || !found {
			// A path running through a non-map is the same "not there" answer as !found, exactly as
			// the comparison in fieldValue treats it.
			continue
		}
		if err := unstructured.SetNestedField(out, value, path...); err != nil {
			return nil, fmt.Errorf("record %s in the snapshot: %w", field, err)
		}
	}
	return out, nil
}

func EncodeSnapshot(s Snapshot) (map[string]string, error) {
	instanceClass, err := json.Marshal(s.InstanceClass)
	if err != nil {
		return nil, fmt.Errorf("serialize InstanceClass snapshot: %w", err)
	}
	provider, err := json.Marshal(s.Provider)
	if err != nil {
		return nil, fmt.Errorf("serialize provider config snapshot: %w", err)
	}
	return map[string]string{
		AppliedInstanceClassAnnotation:  string(instanceClass),
		AppliedProviderConfigAnnotation: string(provider),
		AppliedRolloutIDAnnotation:      s.RolloutID,
		AppliedGenerationAnnotation:     strconv.Itoa(s.Generation),
	}, nil
}

// DecodeSnapshot reads the snapshot back. It reports false when there is none — which is both the
// v1-era object being adopted for the first time and, deliberately, an unparsable snapshot: a
// corrupted annotation must lead to re-adoption with the current values, never to a rollout.
func DecodeSnapshot(annotations map[string]string) (Snapshot, bool) {
	raw, ok := annotations[AppliedInstanceClassAnnotation]
	if !ok {
		return Snapshot{}, false
	}
	instanceClass := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &instanceClass); err != nil {
		return Snapshot{}, false
	}

	// The provider config is read on the same terms: unreadable means re-adoption with the current
	// values, never a rollout.
	provider := map[string]any{}
	if rawProvider, ok := annotations[AppliedProviderConfigAnnotation]; ok {
		if err := json.Unmarshal([]byte(rawProvider), &provider); err != nil {
			return Snapshot{}, false
		}
	}

	// A missing or malformed generation counts as 0: an object adopted before this annotation
	// existed still names its successor gen1.
	generation, _ := strconv.Atoi(annotations[AppliedGenerationAnnotation])
	if generation < 0 {
		generation = 0
	}

	return Snapshot{
		InstanceClass: instanceClass,
		Provider:      provider,
		RolloutID:     annotations[AppliedRolloutIDAnnotation],
		Generation:    generation,
	}, true
}
