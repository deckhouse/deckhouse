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

package v1alpha1

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestDVPInstanceClassImplementsContract(t *testing.T) {
	t.Parallel()

	var _ cpapi.InstanceClassObject = (*DVPInstanceClass)(nil)

	class := &DVPInstanceClass{}
	class.Kind = "DVPInstanceClass"
	class.Name = "master-dvp"

	if class.GroupVersionKind().Kind != "DVPInstanceClass" {
		t.Fatalf("GetKind() = %q, want DVPInstanceClass", class.GroupVersionKind().Kind)
	}
	if class.GetName() != "master-dvp" {
		t.Fatalf("GetName() = %q, want master-dvp", class.GetName())
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() != nil, want nil for zero-value etcdDisk")
	}

	class.Spec.EtcdDisk.Size = "10Gi"
	if class.GetEtcdDisk() == nil {
		t.Fatal("GetEtcdDisk() = nil, want the etcd disk with EtcdDisk.Size set")
	}

	if class.GetNodeGroupConsumers() != nil {
		t.Fatal("GetNodeGroupConsumers() must be nil for DVPInstanceClass")
	}
}

func TestDVPInstanceClassContractNilSafe(t *testing.T) {
	t.Parallel()

	var class *DVPInstanceClass

	if class.GroupVersionKind().Kind != DVPInstanceClassKind {
		t.Fatalf("GetKind() on nil = %q, want %s (type-level GVK)", class.GroupVersionKind().Kind, DVPInstanceClassKind)
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() on nil must be nil")
	}
	if class.GetNodeGroupConsumers() != nil {
		t.Fatal("GetNodeGroupConsumers() on nil must be nil")
	}
}
