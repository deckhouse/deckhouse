//go:build !integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"testing"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func TestResourceBaseName(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"simple", "main", "d8-waypoint-main"},
		{"empty", "", "d8-waypoint-"},
		{"with_dashes", "my-app", "d8-waypoint-my-app"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resourceBaseName(tc.input)
			if got != tc.want {
				t.Errorf("resourceBaseName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestInstanceLabels(t *testing.T) {
	inst := newInstance("main", "ns")

	got := instanceLabels(inst)

	wantKeys := map[string]string{
		AppLabelKey:              AppLabelValue,
		WaypointInstanceLabelKey: "main",
		HeritageLabelKey:         HeritageLabelValue,
	}

	for k, want := range wantKeys {
		if got[k] != want {
			t.Errorf("instanceLabels[%q] = %q, want %q", k, got[k], want)
		}
	}

	for k, v := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Errorf("unexpected label %q = %q in instance labels", k, v)
		}
	}

	if len(got) != len(wantKeys) {
		t.Errorf("instanceLabels len = %d, want %d", len(got), len(wantKeys))
	}
}

func TestIstioLabels_DefaultsAndOverrides(t *testing.T) {
	cases := []struct {
		name        string
		inst        *networkv1alpha1.WaypointInstance
		revision    string
		networkName string
		wantKeys    map[string]string
	}{
		{
			name:        "defaults waypointFor to All",
			inst:        newInstance("main", "ns"),
			revision:    "v1x25x2",
			networkName: "test-network",
			wantKeys: map[string]string{
				"gateway.istio.io/managed":  "istio.io-mesh-controller",
				"istio.io/rev":              "v1x25x2",
				"istio.io/waypoint-for":     "all",
				"topology.istio.io/network": "test-network",
			},
		},
		{
			name:        "explicit waypointFor=Service",
			inst:        newInstance("main", "ns", withWaypointFor("Service")),
			revision:    "v1x24x1",
			networkName: "prod-net",
			wantKeys: map[string]string{
				"gateway.istio.io/managed":  "istio.io-mesh-controller",
				"istio.io/rev":              "v1x24x1",
				"istio.io/waypoint-for":     "service",
				"topology.istio.io/network": "prod-net",
			},
		},
		{
			name:        "explicit waypointFor=Workload",
			inst:        newInstance("main", "ns", withWaypointFor("Workload")),
			revision:    "v1x25x2",
			networkName: "staging-net",
			wantKeys: map[string]string{
				"gateway.istio.io/managed":  "istio.io-mesh-controller",
				"istio.io/rev":              "v1x25x2",
				"istio.io/waypoint-for":     "workload",
				"topology.istio.io/network": "staging-net",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := istioLabels(tc.inst, tc.revision, tc.networkName)
			for k, want := range tc.wantKeys {
				if got[k] != want {
					t.Errorf("istioLabels[%q] = %q, want %q", k, got[k], want)
				}
			}
			for k, v := range got {
				if _, ok := tc.wantKeys[k]; !ok {
					t.Errorf("unexpected label %q = %q in istio labels", k, v)
				}
			}
			if len(got) != len(tc.wantKeys) {
				t.Errorf("istioLabels len = %d, want %d", len(got), len(tc.wantKeys))
			}
		})
	}
}

func TestPodTemplateLabels_StaticKeys(t *testing.T) {
	inst := newInstance("main", "ns")
	got := podTemplateLabels(inst, "v1x25x2", "test-network")

	wantKeys := map[string]string{
		AppLabelKey:                              AppLabelValue,
		WaypointInstanceLabelKey:                 "main",
		HeritageLabelKey:                         HeritageLabelValue,
		"gateway.istio.io/managed":               "istio.io-mesh-controller",
		"istio.io/rev":                           "v1x25x2",
		"istio.io/waypoint-for":                  "all",
		"topology.istio.io/network":              "test-network",
		"istio.io/dataplane-mode":                "none",
		"sidecar.istio.io/inject":                "false",
		"service.istio.io/canonical-name":        "d8-waypoint-main",
		"service.istio.io/canonical-revision":    "latest",
		"gateway.networking.k8s.io/gateway-name": "d8-waypoint-main",
	}

	for k, want := range wantKeys {
		if got[k] != want {
			t.Errorf("podTemplateLabels[%q] = %q, want %q", k, got[k], want)
		}
	}

	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Errorf("unexpected label %q = %q in pod template labels", k, got[k])
		}
	}
}
