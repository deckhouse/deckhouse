/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
)

func TestAbsFolderPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		datacenter       string
		discoveredFolder string
		cfgFolder        *string
		want             string
	}{
		{
			name:             "uses config folder over discovered",
			datacenter:       "DC1",
			discoveredFolder: "discovered/path",
			cfgFolder:        strPtr("cfg/path"),
			want:             "/DC1/vm/cfg/path",
		},
		{
			name:             "falls back to discovered",
			datacenter:       "DC1",
			discoveredFolder: "discovered/path",
			want:             "/DC1/vm/discovered/path",
		},
		{
			name:       "empty folder returns empty",
			datacenter: "DC1",
			want:       "",
		},
		{
			name:             "strips leading slash from relative folder",
			datacenter:       "DC1",
			discoveredFolder: "/relative",
			want:             "/DC1/vm/relative",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, absFolderPath(tt.datacenter, tt.discoveredFolder, tt.cfgFolder))
		})
	}
}

func TestAbsResourcePoolPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/DC1/host/cluster/Resources", absResourcePoolPath("/DC1/host/cluster", ""))
	assert.Equal(t, "/DC1/host/cluster/Resources/kubernetes-dev", absResourcePoolPath("/DC1/host/cluster", "kubernetes-dev"))
	assert.Equal(t, "/DC1/host/cluster/Resources/kubernetes-dev", absResourcePoolPath("/DC1/host/cluster", "/kubernetes-dev"))
}

func TestDatastoreForZone(t *testing.T) {
	t.Parallel()
	datastores := []cloudDataV1.VsphereDatastore{
		{Zones: []string{"zone-a"}, InventoryPath: "/DC1/datastore/ds-a", Name: "ds-a"},
		{Zones: []string{"zone-b"}, Name: "ds-b"},
	}
	assert.Equal(t, "/DC1/datastore/ds-a", datastoreForZone("zone-a", datastores))
	assert.Equal(t, "ds-b", datastoreForZone("zone-b", datastores))
	assert.Equal(t, "", datastoreForZone("zone-missing", datastores))
}

func TestNodeRoleKeysFromNodeGroups(t *testing.T) {
	t.Parallel()
	items := []unstructured.Unstructured{
		*newNodeGroup("worker", "CloudEphemeral", []string{"zone-a", "zone-b"}),
		*newNodeGroup("static", "Static", []string{"zone-a"}),
		*newNodeGroup("cloud", "Cloud", []string{"zone-a"}),
		*newNodeGroup("worker", "CloudEphemeral", []string{"zone-a"}),
	}
	keys := nodeRoleKeysFromNodeGroups(items)
	assert.ElementsMatch(t, []string{"worker-zone-a", "worker-zone-b", "cloud-zone-a"}, keys)
}

func TestBuildFailureDomainSetsAutoConfigureFalse(t *testing.T) {
	t.Parallel()
	fd := buildFailureDomain("vsphere-zone-a", "region", "region-cat", "zone-a", "zone-cat", "DC1", "/DC1/host/cl", "/DC1/datastore/ds")
	require.Equal(t, "VSphereFailureDomain", fd.GetKind())
	regionAuto, found, err := unstructured.NestedBool(fd.Object, "spec", "region", "autoConfigure")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, regionAuto)
	zoneAuto, found, err := unstructured.NestedBool(fd.Object, "spec", "zone", "autoConfigure")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, zoneAuto)
	_, found, err = unstructured.NestedStringSlice(fd.Object, "spec", "topology", "networks")
	require.NoError(t, err)
	assert.False(t, found, "networks must stay unset")
}

func TestBuildDeploymentZonePlacement(t *testing.T) {
	t.Parallel()
	dz := buildDeploymentZone("zone-a", "vcenter.example", "vsphere-zone-a", "/DC1/vm/folder", "/DC1/host/cl/Resources/rp")
	require.Equal(t, "VSphereDeploymentZone", dz.GetKind())
	folder, found, err := unstructured.NestedString(dz.Object, "spec", "placementConstraint", "folder")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "/DC1/vm/folder", folder)
	rp, found, err := unstructured.NestedString(dz.Object, "spec", "placementConstraint", "resourcePool")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "/DC1/host/cl/Resources/rp", rp)
}

func newNodeGroup(name, nodeType string, zones []string) *unstructured.Unstructured {
	zoneIfaces := make([]interface{}, 0, len(zones))
	for _, z := range zones {
		zoneIfaces = append(zoneIfaces, z)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"nodeType": nodeType,
			"cloudInstances": map[string]interface{}{
				"zones": zoneIfaces,
			},
		},
	}}
}

func strPtr(s string) *string { return &s }
