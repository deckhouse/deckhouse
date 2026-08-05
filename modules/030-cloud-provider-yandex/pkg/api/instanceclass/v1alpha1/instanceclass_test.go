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

func TestYandexInstanceClassImplementsContract(t *testing.T) {
	t.Parallel()

	var _ cpapi.InstanceClassObject = (*YandexInstanceClass)(nil)

	class := &YandexInstanceClass{}
	class.Kind = YandexInstanceClassKind
	class.Name = "master-yandex"

	if class.GroupVersionKind().Kind != YandexInstanceClassKind {
		t.Fatalf("GetKind() = %q, want %s", class.GroupVersionKind().Kind, YandexInstanceClassKind)
	}
	if class.GetName() != "master-yandex" {
		t.Fatalf("GetName() = %q, want master-yandex", class.GetName())
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() != nil, want nil without etcdDiskSizeGB")
	}
	if len(class.GetNodeGroupConsumers()) != 0 {
		t.Fatalf("GetNodeGroupConsumers() = %v, want []", class.GetNodeGroupConsumers())
	}

	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() != nil, want nil")
	}
}

func TestYandexInstanceClassContractNilSafe(t *testing.T) {
	t.Parallel()

	var class *YandexInstanceClass

	if class.GroupVersionKind().Kind != YandexInstanceClassKind {
		t.Fatalf("GetKind() on nil = %q, want %s (type-level GVK)", class.GroupVersionKind().Kind, YandexInstanceClassKind)
	}
	if class.GetEtcdDisk() != nil {
		t.Fatal("GetEtcdDisk() on nil must be nil")
	}
	if class.GetNodeGroupConsumers() != nil {
		t.Fatal("GetNodeGroupConsumers() on nil must be nil")
	}
}
