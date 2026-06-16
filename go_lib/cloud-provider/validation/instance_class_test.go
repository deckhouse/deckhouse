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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateInstanceClassEtcdDiskAttachmentAllowsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: v1.ObjectMeta{Name: "master"},
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
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "master-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: map[string]any{},
				},
			},
			{
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "orphan-dvp"},
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

func TestValidateInstanceClassEtcdDiskAttachmentReportsAllNonMasterConsumers(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: v1.ObjectMeta{Name: "master"},
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
				ObjectMeta: v1.ObjectMeta{Name: "worker-a"},
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
				ObjectMeta: v1.ObjectMeta{Name: "worker-b"},
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
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "shared-dvp"},
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

func TestValidateInstanceClassEtcdDiskAttachmentAllowsMasterOnly(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: v1.ObjectMeta{Name: "master"},
				Spec: cpapi.NodeGroupSpec{
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "master-dvp"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() unexpected errors: %s", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentSkipsOtherKinds(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{
			{
				TypeMeta:   v1.TypeMeta{Kind: "OtherInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "orphan"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q, want skip other kinds", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentRequiresMasterWhenAttached(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		[]cpapi.NodeGroup{
			{
				ObjectMeta: v1.ObjectMeta{Name: "worker"},
				Spec: cpapi.NodeGroupSpec{
					CloudInstances: &cpapi.CloudInstances{
						ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "worker-dvp"},
					},
				},
			},
		},
		[]cpapi.InstanceClass{
			{
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "worker-dvp"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: map[string]any{}},
			},
		},
	))

	if !result.HasErrors() || !strings.Contains(result.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentSkipsNilEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(instanceClassState(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{{ObjectMeta: v1.ObjectMeta{Name: "plain"}}},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q, want no errors", result.Error())
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
			ObjectMeta: v1.ObjectMeta{Name: "master"},
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
		ObjectMeta: v1.ObjectMeta{Name: "orphan-dvp"},
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
			ObjectMeta: v1.ObjectMeta{Name: "master"},
			Spec: cpapi.NodeGroupSpec{
				CloudInstances: &cpapi.CloudInstances{
					ClassReference: &cpapi.ClassReference{Kind: "DVPInstanceClass", Name: "master-dvp"},
				},
			},
		}},
		nil,
	)
	deleted := &cpapi.InstanceClass{ObjectMeta: v1.ObjectMeta{Name: "master-dvp"}}

	result := ValidateInstanceClassDelete(state, "", deleted)
	if !hasViolationCode(result, "instance_class_in_use") {
		t.Fatalf("ValidateInstanceClassDelete() = %q", result.Error())
	}
}
