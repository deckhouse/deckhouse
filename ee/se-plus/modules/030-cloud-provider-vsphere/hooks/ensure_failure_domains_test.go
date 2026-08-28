/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"testing"

	"k8s.io/utils/ptr"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

func TestAbsFolderPath(t *testing.T) {
	tests := []struct {
		name        string
		datacenter  string
		discovered  string
		cfg         *string
		want        string
	}{
		{"discovery only", "DC", "e2e-tests/foo", nil, "/DC/vm/e2e-tests/foo"},
		{"cfg overrides discovery", "DC", "old", ptr.To("new/path"), "/DC/vm/new/path"},
		{"cfg empty string is ignored", "DC", "disc", ptr.To(""), "/DC/vm/disc"},
		{"strips leading slash", "DC", "/e2e-tests/foo", nil, "/DC/vm/e2e-tests/foo"},
		{"empty everything returns empty", "DC", "", nil, ""},
		{"empty everything returns empty even with dc", "DC", "", ptr.To(""), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := absFolderPath(tc.datacenter, tc.discovered, tc.cfg)
			if got != tc.want {
				t.Fatalf("absFolderPath(%q,%q,%v) = %q; want %q", tc.datacenter, tc.discovered, tc.cfg, got, tc.want)
			}
		})
	}
}

func TestAbsResourcePoolPath(t *testing.T) {
	tests := []struct {
		name        string
		clusterPath string
		relativeRP  string
		want        string
	}{
		{"joined", "/DC/host/cl", "kubernetes-dev", "/DC/host/cl/Resources/kubernetes-dev"},
		{"trims leading slash on rp", "/DC/host/cl", "/kubernetes-dev", "/DC/host/cl/Resources/kubernetes-dev"},
		{"empty rp falls back to Resources", "/DC/host/cl", "", "/DC/host/cl/Resources"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := absResourcePoolPath(tc.clusterPath, tc.relativeRP)
			if got != tc.want {
				t.Fatalf("absResourcePoolPath(%q,%q) = %q; want %q", tc.clusterPath, tc.relativeRP, got, tc.want)
			}
		})
	}
}

func TestDatastoreForZone(t *testing.T) {
	dsA := cloudDataV1.VsphereDatastore{Name: "a", InventoryPath: "/DC/datastore/a", Zones: []string{"z1"}}
	dsB := cloudDataV1.VsphereDatastore{Name: "b", InventoryPath: "/DC/datastore/b", Zones: []string{"z1"}}
	dsC := cloudDataV1.VsphereDatastore{Name: "c", Zones: []string{"z2"}} // no InventoryPath
	tests := []struct {
		name string
		zone string
		in   []cloudDataV1.VsphereDatastore
		want string
	}{
		{"picks alphabetically first InventoryPath for tie", "z1", []cloudDataV1.VsphereDatastore{dsB, dsA}, "/DC/datastore/a"},
		{"stable when order flipped", "z1", []cloudDataV1.VsphereDatastore{dsA, dsB}, "/DC/datastore/a"},
		{"falls back to Name when no InventoryPath", "z2", []cloudDataV1.VsphereDatastore{dsC}, "c"},
		{"no match returns empty", "z3", []cloudDataV1.VsphereDatastore{dsA, dsB, dsC}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := datastoreForZone(tc.zone, tc.in)
			if got != tc.want {
				t.Fatalf("datastoreForZone(%q) = %q; want %q", tc.zone, got, tc.want)
			}
		})
	}
}

func TestRfc1123SubdomainName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase alnum", "e2e-zone-1", "e2e-zone-1"},
		{"uppercase folded", "E2E-Zone-1", "e2e-zone-1"},
		{"spaces to dashes", "E2E Zone 1", "e2e-zone-1"},
		{"illegal chars dropped", "zone/1@lab", "zone1lab"},
		{"trims leading/trailing separators", "-.zone-1.-", "zone-1"},
		{"dots preserved", "zone.a.b", "zone.a.b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rfc1123SubdomainName(tc.in); got != tc.want {
				t.Fatalf("rfc1123SubdomainName(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildFailureDomain(t *testing.T) {
	fd := buildFailureDomain("vsphere-z1", "e2e-region", "k8s-region", "z1", "k8s-zone",
		"DatacenterLAB", "/DatacenterLAB/host/cl", "/DatacenterLAB/datastore/ds")

	if got := fd.GetName(); got != "vsphere-z1" {
		t.Fatalf("name = %q; want vsphere-z1", got)
	}
	if got := fd.GetAPIVersion(); got != fdAPIVersion {
		t.Fatalf("apiVersion = %q; want %q", got, fdAPIVersion)
	}
	if got := fd.GetKind(); got != fdKind {
		t.Fatalf("kind = %q; want %q", got, fdKind)
	}
	spec, _ := fd.Object["spec"].(map[string]interface{})
	region, _ := spec["region"].(map[string]interface{})
	zone, _ := spec["zone"].(map[string]interface{})
	topo, _ := spec["topology"].(map[string]interface{})

	// autoConfigure MUST be present as explicit false — CAPV panics on nil dereference otherwise.
	if v, ok := region["autoConfigure"]; !ok || v != false {
		t.Fatalf("region.autoConfigure = %v (ok=%v); want false", v, ok)
	}
	if v, ok := zone["autoConfigure"]; !ok || v != false {
		t.Fatalf("zone.autoConfigure = %v (ok=%v); want false", v, ok)
	}
	// networks MUST NOT be set — see networks-drop rationale in ensure_failure_domains.go.
	if _, ok := topo["networks"]; ok {
		t.Fatalf("topology.networks must be absent, got: %v", topo["networks"])
	}
	if topo["computeCluster"] != "/DatacenterLAB/host/cl" {
		t.Fatalf("topology.computeCluster = %v", topo["computeCluster"])
	}
	if topo["datastore"] != "/DatacenterLAB/datastore/ds" {
		t.Fatalf("topology.datastore = %v", topo["datastore"])
	}
}

func TestBuildDeploymentZone(t *testing.T) {
	dz := buildDeploymentZone("z1", "vcenter.example", "vsphere-z1",
		"/DC/vm/folder", "/DC/host/cl/Resources/rp", dzTypeBase, "")

	if got := dz.GetName(); got != "z1" {
		t.Fatalf("name = %q; want z1", got)
	}
	spec, _ := dz.Object["spec"].(map[string]interface{})
	if spec["failureDomain"] != "vsphere-z1" {
		t.Fatalf("failureDomain = %v", spec["failureDomain"])
	}
	if spec["controlPlane"] != false {
		t.Fatalf("controlPlane = %v; want false", spec["controlPlane"])
	}
	pc, _ := spec["placementConstraint"].(map[string]interface{})
	if pc["folder"] != "/DC/vm/folder" {
		t.Fatalf("folder = %v", pc["folder"])
	}
	if pc["resourcePool"] != "/DC/host/cl/Resources/rp" {
		t.Fatalf("resourcePool = %v", pc["resourcePool"])
	}
	labels := dz.GetLabels()
	if labels[dzTypeLabel] != dzTypeBase {
		t.Fatalf("dz-type label = %q; want %q", labels[dzTypeLabel], dzTypeBase)
	}
	if _, ok := labels[dzNodeGroupLabel]; ok {
		t.Fatalf("node-group label should be omitted for base DZ, got %q", labels[dzNodeGroupLabel])
	}
}

func TestBuildDeploymentZoneOverrideLabels(t *testing.T) {
	dz := buildDeploymentZone("z1-worker-fast", "vcenter.example", "vsphere-z1",
		"/DC/vm/folder", "/DC/host/cl/Resources/prod", dzTypeOverride, "worker-fast")
	labels := dz.GetLabels()
	if labels[dzTypeLabel] != dzTypeOverride {
		t.Fatalf("dz-type label = %q; want %q", labels[dzTypeLabel], dzTypeOverride)
	}
	if labels[dzNodeGroupLabel] != "worker-fast" {
		t.Fatalf("node-group label = %q; want worker-fast", labels[dzNodeGroupLabel])
	}
}

func TestBuildDeploymentZoneOmitsEmptyPlacement(t *testing.T) {
	dz := buildDeploymentZone("z1", "vcenter.example", "vsphere-z1", "", "", dzTypeBase, "")
	spec, _ := dz.Object["spec"].(map[string]interface{})
	pc, _ := spec["placementConstraint"].(map[string]interface{})
	if _, ok := pc["folder"]; ok {
		t.Fatalf("folder should be omitted when empty, got: %v", pc["folder"])
	}
	if _, ok := pc["resourcePool"]; ok {
		t.Fatalf("resourcePool should be omitted when empty, got: %v", pc["resourcePool"])
	}
}

func TestStrPtrOrEmpty(t *testing.T) {
	if got := strPtrOrEmpty(nil); got != "" {
		t.Fatalf("nil = %q; want empty", got)
	}
	if got := strPtrOrEmpty(ptr.To("foo")); got != "foo" {
		t.Fatalf("ptr = %q; want foo", got)
	}
}
