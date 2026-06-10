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
	"encoding/json"
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateInstanceClassEtcdDiskAttachmentRejectsUnattachedEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
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
					EtcdDisk: rawJSONForInstanceClassTest("{}"),
				},
			},
			{
				TypeMeta:   v1.TypeMeta{Kind: "DVPInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "orphan-dvp"},
				Spec: cpapi.InstanceClassSpec{
					EtcdDisk: rawJSONForInstanceClassTest("{}"),
				},
			},
		},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "DVPInstanceClass/orphan-dvp.spec.etcdDisk") {
		t.Fatalf("expected unattached etcdDisk error, got: %s", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentReportsAllNonMasterConsumers(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
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
					EtcdDisk: rawJSONForInstanceClassTest("{}"),
				},
			},
		},
	)

	if len(result.Errors) != 2 {
		t.Fatalf("expected two etcdDisk errors, got %d: %s", len(result.Errors), result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentAllowsMasterOnly(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
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
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: rawJSONForInstanceClassTest("{}")},
			},
		},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() unexpected errors: %s", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentSkipsOtherKinds(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{
			{
				TypeMeta:   v1.TypeMeta{Kind: "OtherInstanceClass"},
				ObjectMeta: v1.ObjectMeta{Name: "orphan"},
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: rawJSONForInstanceClassTest("{}")},
			},
		},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q, want skip other kinds", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentRequiresMasterWhenAttached(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
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
				Spec:       cpapi.InstanceClassSpec{EtcdDisk: rawJSONForInstanceClassTest("{}")},
			},
		},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q", result.Error())
	}
}

func TestValidateInstanceClassEtcdDiskAttachmentSkipsNilEtcdDisk(t *testing.T) {
	t.Parallel()

	result := ValidateInstanceClassEtcdDiskAttachment(
		"DVPInstanceClass",
		nil,
		[]cpapi.InstanceClass{{ObjectMeta: v1.ObjectMeta{Name: "plain"}}},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateInstanceClassEtcdDiskAttachment() = %q, want no errors", result.Error())
	}
}

func rawJSONForInstanceClassTest(value string) *json.RawMessage {
	message := json.RawMessage(value)
	return &message
}
