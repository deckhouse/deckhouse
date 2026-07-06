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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// A non-cloud NodeGroup is valid but never "processed": get_crds skips the whole
// cloud branch, so no checks run, no overlays are emitted and no error is set.
func TestRunCloudChecks_NonCloud(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:     v1.NodeTypeStatic,
		KindInUse:    "D8TestInstanceClass",
		ClassRefKind: "Wrong",
	})
	assert.False(t, res.Processed)
	assert.Empty(t, res.Error, "non-cloud NG must not run checks or set an error")
}

// A CloudEphemeral NG with an unresolvable kind (empty kindInUse) is skipped the
// same way: get_crds's cloud branch is gated on kindInUse != "".
func TestRunCloudChecks_NoKindInUse(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:     v1.NodeTypeCloudEphemeral,
		KindInUse:    "",
		ClassRefKind: "D8TestInstanceClass",
	})
	assert.False(t, res.Processed)
	assert.Empty(t, res.Error)
}

// check #1: classReference.kind must equal the cluster's allowed kind.
func TestRunCloudChecks_KindMismatch(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:     v1.NodeTypeCloudEphemeral,
		KindInUse:    "D8TestInstanceClass",
		ClassRefKind: "OtherInstanceClass",
		ClassRefName: "proper1",
	})
	assert.False(t, res.Processed)
	assert.Equal(t,
		"Invalid classReference.kind 'OtherInstanceClass'. Expected 'D8TestInstanceClass'. Please update the NodeGroup to use the correct instance class kind.",
		res.Error)
}

// check #2: the referenced instance class must exist.
func TestRunCloudChecks_UnknownClass(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "missing",
		KnownClassNames: []string{"proper1", "other"},
	})
	assert.False(t, res.Processed)
	assert.Equal(t,
		"Instance class 'missing' of type 'D8TestInstanceClass' not found. Please create the required instance class or update the NodeGroup to reference an existing one.",
		res.Error)
}

// check #3: scale-from-zero (min==0 && max>0) requires a resolvable capacity.
func TestRunCloudChecks_ScaleFromZeroCapacityFailure(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "proper1",
		KnownClassNames: []string{"proper1"},
		MinPerZone:      0,
		MaxPerZone:      3,
		CapacityErr:     errors.New("not found"),
	})
	assert.False(t, res.Processed)
	assert.Equal(t,
		"Capacity calculation failed for instance class 'D8TestInstanceClass'. The instance type is not found in built-in types and no capacity is set. ScaleFromZero will not work. Please set capacity in the D8TestInstanceClass 'proper1' or use a supported instance type.",
		res.Error)
}

// check #3 does not fire when the group is not scaling from zero, even if the
// capacity calc reported an error (get_crds only consults it for min==0&&max>0).
func TestRunCloudChecks_CapacityErrorIgnoredWhenNotScaleFromZero(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "proper1",
		KnownClassNames: []string{"proper1"},
		MinPerZone:      1,
		MaxPerZone:      3,
		CapacityErr:     errors.New("not found"),
	})
	assert.True(t, res.Processed)
	assert.Empty(t, res.Error)
}

// check #4: every spec zone must be a known default zone; the error lists the
// offending zones and the sorted set of available zones.
func TestRunCloudChecks_InvalidZones(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "proper1",
		KnownClassNames: []string{"proper1"},
		SpecZones:       []string{"z9"},
		DefaultZones:    []string{"b", "a", "c"},
	})
	assert.False(t, res.Processed)
	assert.Equal(t,
		"Invalid zones specified: [z9]. Available zones: [a b c]. Please update the NodeGroup to use valid zones.",
		res.Error)
}

// check #4 is skipped when there are no default zones to validate against.
func TestRunCloudChecks_NoDefaultZonesSkipsZoneCheck(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "proper1",
		KnownClassNames: []string{"proper1"},
		SpecZones:       []string{"whatever"},
		DefaultZones:    nil,
	})
	assert.True(t, res.Processed)
	assert.Empty(t, res.Error)
}

// All checks pass: the NG is processed and receives cloud overlays.
func TestRunCloudChecks_AllPass(t *testing.T) {
	res := RunCloudChecks(CloudCheckInput{
		NodeType:        v1.NodeTypeCloudEphemeral,
		KindInUse:       "D8TestInstanceClass",
		ClassRefKind:    "D8TestInstanceClass",
		ClassRefName:    "proper1",
		KnownClassNames: []string{"proper1"},
		MinPerZone:      1,
		MaxPerZone:      3,
		SpecZones:       []string{"a", "c"},
		DefaultZones:    []string{"a", "b", "c"},
	})
	assert.True(t, res.Processed)
	assert.Empty(t, res.Error)
}
