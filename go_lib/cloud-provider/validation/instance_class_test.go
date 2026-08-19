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

package validation

import (
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	testprovider "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
)

func TestValidateInstanceClassesEtcdDiskAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
							Name: "master-dvp",
						},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("master-dvp", true),
			testInstanceClass("orphan-dvp", true),
		},
	))

	if result.HasErrors() {
		t.Fatalf("expected unattached etcdDisk allowed, got: %s", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskReportsAllNonMasterConsumers(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker-a"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker-b"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "TestInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("shared-dvp", true),
		},
	))

	if len(result.Errors()) != 1 {
		t.Fatalf("expected one deduplicated etcdDisk error, got %d: %s", len(result.Errors()), result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskAllowsMasterOnly(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "master-dvp"},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("master-dvp", true),
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() unexpected errors: %s", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskSkipsOtherKinds(t *testing.T) {
	t.Parallel()

	// A NodeGroup referencing another provider's kind is not a consumer of this
	// provider's class, even when the names collide: without the kind filter the
	// worker below would make "shared-name" look attached to a non-master NodeGroup.
	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "OtherProviderInstanceClass", Name: "shared-name"},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("shared-name", true),
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q, want other kinds skipped", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskCountsOwnKindConsumers(t *testing.T) {
	t.Parallel()

	// Same fixture as above with the reference pointing at this provider's kind:
	// the worker becomes a consumer and the etcdDisk is rejected. Together with
	// TestValidateInstanceClassesEtcdDiskSkipsOtherKinds this pins the kind filter.
	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "shared-name"},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("shared-name", true),
		},
	))

	if !hasViolationCode(result, CodeEtcdDiskForbiddenForNonMaster) {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q, want %s", result.Error(), CodeEtcdDiskForbiddenForNonMaster)
	}
}

func TestValidateInstanceClassesEtcdDiskRequiresMasterWhenAttached(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "worker-dvp"},
					},
				},
			},
		},
		[]*testprovider.InstanceClass{
			testInstanceClass("worker-dvp", true),
		},
	))

	if !result.HasErrors() || !strings.Contains(result.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskSkipsNilEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		nil,
		[]*testprovider.InstanceClass{testInstanceClass("plain", false)},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassDeleteNilClass(t *testing.T) {
	t.Parallel()

	state := instanceClassState(nil, nil)
	if result := ValidateInstanceClassDeletion(state, (*testprovider.InstanceClass)(nil)); result.HasErrors() {
		t.Fatalf("ValidateInstanceClassDeletion() = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassDeleteInUseByNodeGroup(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		nil,
	)

	deleted := testInstanceClass("master-dvp", false)
	result := ValidateInstanceClassDeletion(state, deleted)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDeletion() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteInUseByCloudEphemeralNodeGroup(t *testing.T) {
	t.Parallel()

	// Deletion must be blocked whatever the consumer's nodeType: a CloudEphemeral NodeGroup
	// pointing at the class would lose its template just as a CloudPermanent one would.
	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudEphemeral,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "worker-dvp"},
				},
			},
		}},
		nil,
	)

	result := ValidateInstanceClassDeletion(state, testInstanceClass("worker-dvp", false))
	if !hasViolationCode(result, CodeInstanceClassInUse) {
		t.Fatalf("ValidateInstanceClassDeletion() = %q, want %s", result.Error(), CodeInstanceClassInUse)
	}
}

func TestValidateInstanceClassDeleteWithStatusConsumers(t *testing.T) {
	t.Parallel()

	state := instanceClassState(nil, nil)
	deleted := testInstanceClass("orphan-dvp", false)
	deleted.Status.NodeGroupConsumers = []string{"worker"}

	result := ValidateInstanceClassDeletion(state, deleted)
	if !hasViolationCode(result, "instance_class_has_consumers") {
		t.Fatalf("ValidateInstanceClassDeletion() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteUsesDeletedClassName(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		nil,
	)
	deleted := testInstanceClass("master-dvp", false)

	result := ValidateInstanceClassDeletion(state, deleted)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDeletion() = %q", result.Error())
	}
}

func TestValidateMasterInstanceClassRequiresEtcdDisk(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		[]*testprovider.InstanceClass{testInstanceClass("master-dvp", false)},
	)

	result := ValidateInstanceClassesEtcdDisk(state)
	if !hasViolationCode(result, "master_etcd_disk_required") {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q", result.Error())
	}

	result = ValidateNodeGroupsClassReference(state, true)
	if result.HasErrors() {
		t.Fatalf("ValidateNodeGroupsClassReference() unexpected errors: %s", result.Error())
	}
}

func TestValidateMasterInstanceClassRequiresExistingClass(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "missing-dvp"},
				},
			},
		}},
		nil,
	)

	result := ValidateNodeGroupsClassReference(state, true)
	if !hasViolationCode(result, "instance_class_not_found") {
		t.Fatalf("ValidateNodeGroupsClassReference() = %q", result.Error())
	}

	result = ValidateInstanceClassesEtcdDisk(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() unexpected errors: %s", result.Error())
	}
}

func TestValidateMasterInstanceClassAllowsConfiguredMaster(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				NodeType: cpapi.NodeTypeCloudPermanent,
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "TestInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		[]*testprovider.InstanceClass{testInstanceClass("master-dvp", true)},
	)

	for name, validate := range map[string]func(*testState) cpvalapi.Result{
		"ValidateNodeGroupsClassReference": func(s *testState) cpvalapi.Result { return ValidateNodeGroupsClassReference(s, true) },
		"ValidateInstanceClassesEtcdDisk":  func(s *testState) cpvalapi.Result { return ValidateInstanceClassesEtcdDisk(s) },
	} {
		result := validate(state)
		if result.HasErrors() {
			t.Fatalf("%s() unexpected errors: %s", name, result.Error())
		}
	}
}
