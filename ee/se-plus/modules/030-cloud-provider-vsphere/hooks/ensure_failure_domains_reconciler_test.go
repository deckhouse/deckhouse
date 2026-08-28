/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
)

// fakeSnapshot / fakeSnapshots implement the pkg.Snapshots interface backed by pre-marshaled
// JSON, so a test can hand desiredOverrideDZs the same payload the real shell-operator
// binding produces from FilterFunc. Using a real JSON round-trip (rather than reflecting
// into UnmarshalTo directly) exercises the path production code takes and catches struct
// tag mistakes.
type fakeSnapshot struct {
	data []byte
}

func (f fakeSnapshot) UnmarshalTo(v any) error {
	return json.Unmarshal(f.data, v)
}

func (f fakeSnapshot) String() string {
	return string(f.data)
}

type fakeSnapshots map[string][]sdkpkg.Snapshot

func (m fakeSnapshots) Get(k string) []sdkpkg.Snapshot { return m[k] }

// newSnaps marshals each entry in items to JSON and buckets them under key. The generic
// helper keeps tests readable: caller passes typed struct literals, the helper serializes.
func newSnaps(t *testing.T, entries map[string][]any) fakeSnapshots {
	t.Helper()
	out := fakeSnapshots{}
	for key, items := range entries {
		for _, item := range items {
			data, err := json.Marshal(item)
			if err != nil {
				t.Fatalf("marshal snapshot item for %q: %v", key, err)
			}
			out[key] = append(out[key], fakeSnapshot{data: data})
		}
	}
	return out
}

// namesSorted returns dz names extracted from the reconciler output in a stable order for
// diffing. desiredOverrideDZs itself is order-agnostic (map iteration), so tests compare
// on sorted names.
func namesSorted(desired map[string]desiredOverrideDZ) []string {
	out := make([]string, 0, len(desired))
	for k := range desired {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestDesiredOverrideDZsEmpty(t *testing.T) {
	snaps := newSnaps(t, nil)
	got, err := desiredOverrideDZs(snaps, []string{"east", "west"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty desired set, got %v", got)
	}
}

// An InstanceClass without a resourcePool override contributes nothing: the NG falls back
// to the baseline DZ that the zone-loop creates elsewhere.
func TestDesiredOverrideDZsNoResourcePool(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "worker", ResourcePool: ""},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "worker",
				InstanceClassKind: instanceClassKind,
				InstanceClassName: "worker",
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no overrides when resourcePool is empty, got %v", namesSorted(got))
	}
}

func TestDesiredOverrideDZsOnePerZone(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "worker-fast", ResourcePool: "/DC/host/cl/Resources/prod"},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "worker-fast",
				InstanceClassKind: instanceClassKind,
				InstanceClassName: "worker-fast",
				Zones:             []string{"east", "west"},
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east", "west", "north"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"east-worker-fast", "west-worker-fast"}
	if diff := namesSorted(got); !reflect.DeepEqual(diff, want) {
		t.Fatalf("names diff: got %v, want %v", diff, want)
	}
	dz := got["east-worker-fast"]
	if dz.ResourcePool != "/DC/host/cl/Resources/prod" {
		t.Fatalf("resourcePool = %q; want the IC override", dz.ResourcePool)
	}
	if dz.FDName != "vsphere-east" {
		t.Fatalf("FDName = %q; want vsphere-east", dz.FDName)
	}
}

// NG.Spec.Zones empty means "all provider zones" — pccZones is the fallback.
func TestDesiredOverrideDZsZonesDefaultToPCC(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "worker-fast", ResourcePool: "/DC/host/cl/Resources/prod"},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "worker-fast",
				InstanceClassKind: instanceClassKind,
				InstanceClassName: "worker-fast",
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east", "west"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"east-worker-fast", "west-worker-fast"}
	if diff := namesSorted(got); !reflect.DeepEqual(diff, want) {
		t.Fatalf("names diff: got %v, want %v", diff, want)
	}
}

// A NG referencing a non-vsphere InstanceClass is ignored entirely.
func TestDesiredOverrideDZsSkipsOtherKinds(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "worker", ResourcePool: "/rp"},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "worker",
				InstanceClassKind: "OpenStackInstanceClass",
				InstanceClassName: "worker",
				Zones:             []string{"east"},
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no overrides for non-vsphere NG, got %v", namesSorted(got))
	}
}

// A NG referencing an InstanceClass that isn't in the snapshot (e.g. it was deleted before
// the NG event fired) is ignored — the reconciler needs the IC.spec.resourcePool value to
// build a DZ, so a missing IC is a no-op, not an error.
func TestDesiredOverrideDZsIgnoresMissingIC(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "worker",
				InstanceClassKind: instanceClassKind,
				InstanceClassName: "missing-ic",
				Zones:             []string{"east"},
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want no overrides when referenced IC is missing, got %v", namesSorted(got))
	}
}

// Two NGs pointing at the same IC each generate their own set of override DZs, because DZ
// identity is per (zone, NG) not per (zone, IC). This gives two NGs pinned to the same
// resource pool distinct DZs, so an operator can point one at a different pool later
// without a rolling update across every NG that shared the IC.
func TestDesiredOverrideDZsTwoNGsSameIC(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "worker", ResourcePool: "/rp"},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name: "a", InstanceClassKind: instanceClassKind, InstanceClassName: "worker",
				Zones: []string{"east"},
			},
			nodeGroupSnapshot{
				Name: "b", InstanceClassKind: instanceClassKind, InstanceClassName: "worker",
				Zones: []string{"east"},
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, []string{"east"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"east-a", "east-b"}
	if diff := namesSorted(got); !reflect.DeepEqual(diff, want) {
		t.Fatalf("names diff: got %v, want %v", diff, want)
	}
}

// Sanitization is applied to both parts of the compound name — a raw NG name with spaces
// or uppercase would 422 on Create; the hook must sanitize before writing.
func TestDesiredOverrideDZsSanitizesBothParts(t *testing.T) {
	snaps := newSnaps(t, map[string][]any{
		"vsphere-instance-classes": {
			instanceClassSnapshot{Name: "Worker Fast", ResourcePool: "/rp"},
		},
		"node-groups": {
			nodeGroupSnapshot{
				Name:              "Worker Fast",
				InstanceClassKind: instanceClassKind,
				InstanceClassName: "Worker Fast",
				Zones:             []string{"E2E Zone 1"},
			},
		},
	})
	got, err := desiredOverrideDZs(snaps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"e2e-zone-1-worker-fast"}
	if diff := namesSorted(got); !reflect.DeepEqual(diff, want) {
		t.Fatalf("names diff: got %v, want %v", diff, want)
	}
	dz := got["e2e-zone-1-worker-fast"]
	if dz.FDName != "vsphere-e2e-zone-1" {
		t.Fatalf("FDName = %q; want sanitized FD name", dz.FDName)
	}
}
