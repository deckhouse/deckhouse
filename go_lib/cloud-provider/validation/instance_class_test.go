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
)

func TestValidateInstanceClassesEtcdDiskAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "DVPInstanceClass",
							Name: "master-dvp",
						},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "orphan-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
		},
	))

	if result.HasErrors() {
		t.Fatalf("expected unattached etcdDisk allowed, got: %s", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskReportsAllNonMasterConsumers(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					NodeType: cpapi.NodeTypeCloudPermanent,
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{
							Kind: "DVPInstanceClass",
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
							Kind: "DVPInstanceClass",
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
							Kind: "DVPInstanceClass",
							Name: "shared-dvp",
						},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "shared-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
		},
	))

	if len(result.Errors()) != 1 {
		t.Fatalf("expected one deduplicated etcdDisk error, got %d: %s", len(result.Errors()), result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskAllowsMasterOnly(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() unexpected errors: %s", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskSkipsOtherKinds(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "OtherInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "orphan"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q, want skip other kinds", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskRequiresMasterWhenAttached(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: cpapi.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "worker-dvp"},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   cpapi.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: cpapi.ObjectMeta{Name: "worker-dvp"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if !result.HasErrors() || !strings.Contains(result.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q", result.Error())
	}
}

func TestValidateInstanceClassesEtcdDiskSkipsNilEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassesEtcdDisk(instanceClassState(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{{ObjectMeta: cpapi.ObjectMeta{Name: "plain"}}},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassDeleteEmptyName(t *testing.T) {
	t.Parallel()

	state := instanceClassState("DVPInstanceClass", nil, nil)
	if result := ValidateInstanceClassDelete(state, "", nil); result.HasErrors() {
		t.Fatalf("ValidateInstanceClassDelete() = %q, want no errors", result.Error())
	}
}

func TestValidateInstanceClassDeleteInUseByNodeGroup(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		nil,
	)

	result := ValidateInstanceClassDelete(state, "master-dvp", nil)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteWithStatusConsumers(t *testing.T) {
	t.Parallel()

	state := instanceClassState("DVPInstanceClass", nil, nil)
	deleted := &cpapi.InstanceClass{
		ObjectMeta: cpapi.ObjectMeta{Name: "orphan-dvp"},
		Status:     cpapi.InstanceClassStatus{NodeGroupConsumers: []any{"worker"}},
	}

	result := ValidateInstanceClassDelete(state, "", deleted)
	if !hasViolationCode(result, "instance_class_has_consumers") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func TestValidateInstanceClassDeleteUsesDeletedClassName(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		nil,
	)
	deleted := &cpapi.InstanceClass{ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"}}

	result := ValidateInstanceClassDelete(state, "", deleted)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}

func TestValidateMasterInstanceClassRequiresEtcdDisk(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		[]cpapi.InstanceClass{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
		}},
	)

	result := ValidateInstanceClassesEtcdDisk(state)
	if !hasViolationCode(result, "master_etcd_disk_required") {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() = %q", result.Error())
	}

	result = ValidateMasterInstanceClassReference(state)
	if result.HasErrors() {
		t.Fatalf("ValidateMasterInstanceClassReference() unexpected errors: %s", result.Error())
	}
}

func TestValidateMasterInstanceClassRequiresExistingClass(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "missing-dvp"},
				},
			},
		}},
		nil,
	)

	result := ValidateMasterInstanceClassReference(state)
	if !hasViolationCode(result, "master_instance_class_not_found") {
		t.Fatalf("ValidateMasterInstanceClassReference() = %q", result.Error())
	}

	result = ValidateInstanceClassesEtcdDisk(state)
	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassesEtcdDisk() unexpected errors: %s", result.Error())
	}
}

func TestValidateMasterInstanceClassAllowsConfiguredMaster(t *testing.T) {
	t.Parallel()

	state := instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		[]cpapi.InstanceClass{{
			ObjectMeta: cpapi.ObjectMeta{Name: "master-dvp"},
			Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{"size": "10Gi"}},
		}},
	)

	for name, validate := range map[string]func(*State) Result{
		"ValidateMasterInstanceClassReference": ValidateMasterInstanceClassReference,
		"ValidateInstanceClassesEtcdDisk":  ValidateInstanceClassesEtcdDisk,
	} {
		result := validate(state)
		if result.HasErrors() {
			t.Fatalf("%s() unexpected errors: %s", name, result.Error())
		}
	}
}
