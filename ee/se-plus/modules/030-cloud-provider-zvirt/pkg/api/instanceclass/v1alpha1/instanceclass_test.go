/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v1alpha1

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestGroupVersionKind(t *testing.T) {
	want := cpapi.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ZvirtInstanceClass"}

	if got := (&ZvirtInstanceClass{}).GroupVersionKind(); got != want {
		t.Fatalf("GroupVersionKind() = %v, want %v", got, want)
	}
}

// The frozen v1alpha1 schema has no etcd disk field, so the class never reports one.
func TestGetEtcdDiskIsAlwaysNil(t *testing.T) {
	var absent *ZvirtInstanceClass

	if got := absent.GetEtcdDisk(); got != nil {
		t.Fatalf("GetEtcdDisk() on a nil receiver = %v, want nil", got)
	}

	if got := (&ZvirtInstanceClass{Spec: InstanceClassSpec{NumCPUs: 4}}).GetEtcdDisk(); got != nil {
		t.Fatalf("GetEtcdDisk() = %v, want nil", got)
	}
}

func TestGetNodeGroupConsumers(t *testing.T) {
	tests := []struct {
		name  string
		class *ZvirtInstanceClass
		want  []string
	}{
		{name: "nil receiver", class: nil, want: nil},
		{name: "no consumers", class: &ZvirtInstanceClass{}, want: nil},
		{
			name:  "with consumers",
			class: &ZvirtInstanceClass{Status: InstanceClassStatus{NodeGroupConsumers: []string{"worker"}}},
			want:  []string{"worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.class.GetNodeGroupConsumers()
			if len(got) != len(tt.want) {
				t.Fatalf("GetNodeGroupConsumers() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("GetNodeGroupConsumers()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
