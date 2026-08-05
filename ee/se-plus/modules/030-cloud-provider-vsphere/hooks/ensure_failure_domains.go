/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	v1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// CAPV VSphereFailureDomain and VSphereDeploymentZone are created once per zone with
// an immutable spec — the validating webhook rejects any subsequent patch/update. This
// hook is create-if-not-exists on purpose: it never rewrites live resources, so an admin
// can hand-tune a FailureDomain without the hook fighting them back on the next reconcile.
//
// ComputeCluster is not present in providerDiscoveryData and cannot be derived from local
// state (zone tags live in vCenter), so the hook opens a vSphere session and resolves
// `zoneTagCategory` → ClusterComputeResource.inventoryPath directly. This keeps the
// discoverer image untouched at the cost of a vSphere round-trip per reconcile.

const (
	fdAPIVersion = "infrastructure.cluster.x-k8s.io/v1beta1"
	fdKind       = "VSphereFailureDomain"
	dzKind       = "VSphereDeploymentZone"
	fdNamePrefix = "vsphere-"
)

var (
	fdGVR = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspherefailuredomains"}
	dzGVR = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspheredeploymentzones"}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/cloud-provider-vsphere/ensure-failure-domains",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
}, dependency.WithExternalDependencies(ensureFailureDomains))

func ensureFailureDomains(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	pccJSON := input.Values.Get("cloudProviderVsphere.internal.providerClusterConfiguration").String()
	if pccJSON == "" || pccJSON == "null" {
		return nil
	}
	var pcc v1.VsphereProviderClusterConfiguration
	if err := json.Unmarshal([]byte(pccJSON), &pcc); err != nil {
		return fmt.Errorf("unmarshal providerClusterConfiguration: %w", err)
	}
	if pcc.Zones == nil || len(*pcc.Zones) == 0 {
		return nil
	}
	if pcc.Provider == nil || pcc.Provider.Server == nil || pcc.Provider.Username == nil || pcc.Provider.Password == nil {
		return nil
	}
	if pcc.ZoneTagCategory == nil || *pcc.ZoneTagCategory == "" {
		return nil
	}

	ddJSON := input.Values.Get("cloudProviderVsphere.internal.providerDiscoveryData").String()
	var dd cloudDataV1.VsphereCloudDiscoveryData
	if ddJSON != "" && ddJSON != "null" {
		if err := json.Unmarshal([]byte(ddJSON), &dd); err != nil {
			return fmt.Errorf("unmarshal providerDiscoveryData: %w", err)
		}
	}
	if dd.Datacenter == "" {
		input.Logger.Warn("providerDiscoveryData.datacenter is empty, skipping FailureDomain ensure")
		return nil
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	dyn := k8sClient.Dynamic()

	missing, err := zonesMissingResources(ctx, dyn, *pcc.Zones)
	if err != nil {
		return fmt.Errorf("check existing FD/DZ: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	zoneClusters, err := resolveZoneComputeClusters(ctx, &pcc, dd.Datacenter, missing)
	if err != nil {
		return fmt.Errorf("resolve compute clusters for zones %v: %w", missing, err)
	}

	networks := networksFromConfig(&pcc)
	folder := absFolderPath(dd.Datacenter, dd.VMFolderPath, pcc.VMFolderPath)
	region := strPtrOrEmpty(pcc.Region)
	regionTagCategory := strPtrOrEmpty(pcc.RegionTagCategory)
	zoneTagCategory := *pcc.ZoneTagCategory
	server := *pcc.Provider.Server

	for _, zone := range missing {
		clusterPath, ok := zoneClusters[zone]
		if !ok {
			input.Logger.Warn("skip zone: ClusterComputeResource not resolved", "zone", zone)
			continue
		}
		datastore := datastoreForZone(zone, dd.Datastores)
		if datastore == "" {
			input.Logger.Warn("skip zone: no datastore tagged for it", "zone", zone)
			continue
		}
		resourcePool := absResourcePoolPath(clusterPath, dd.ResourcePoolPath)

		fd := buildFailureDomain(fdNamePrefix+zone, region, regionTagCategory, zone, zoneTagCategory, dd.Datacenter, clusterPath, datastore, networks)
		dz := buildDeploymentZone(zone, server, fdNamePrefix+zone, folder, resourcePool)
		input.PatchCollector.Create(fd)
		input.PatchCollector.Create(dz)
		input.Logger.Info("created VSphereFailureDomain and VSphereDeploymentZone",
			"zone", zone, "computeCluster", clusterPath, "datastore", datastore)
	}

	return nil
}

func zonesMissingResources(ctx context.Context, dyn dynamic.Interface, zones []string) ([]string, error) {
	fdList, err := dyn.Resource(fdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list VSphereFailureDomains: %w", err)
	}
	dzList, err := dyn.Resource(dzGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list VSphereDeploymentZones: %w", err)
	}
	fdNames := make(map[string]struct{}, len(fdList.Items))
	for _, o := range fdList.Items {
		fdNames[o.GetName()] = struct{}{}
	}
	dzNames := make(map[string]struct{}, len(dzList.Items))
	for _, o := range dzList.Items {
		dzNames[o.GetName()] = struct{}{}
	}
	var missing []string
	for _, zone := range zones {
		_, hasFD := fdNames[fdNamePrefix+zone]
		_, hasDZ := dzNames[zone]
		if !hasFD || !hasDZ {
			missing = append(missing, zone)
		}
	}
	return missing, nil
}

func resolveZoneComputeClusters(ctx context.Context, pcc *v1.VsphereProviderClusterConfiguration, datacenter string, zones []string) (map[string]string, error) {
	server := *pcc.Provider.Server
	username := *pcc.Provider.Username
	password := *pcc.Provider.Password
	insecure := false
	if pcc.Provider.Insecure != nil {
		insecure = *pcc.Provider.Insecure
	}

	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", server))
	if err != nil {
		return nil, fmt.Errorf("build vsphere URL: %w", err)
	}
	u.User = url.UserPassword(username, password)

	c, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("govmomi login: %w", err)
	}
	defer func() { _ = c.Logout(ctx) }()

	restC := rest.NewClient(c.Client)
	if err := restC.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("vsphere rest login: %w", err)
	}
	defer func() { _ = restC.Logout(ctx) }()

	finder := find.NewFinder(c.Client, true)
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return nil, fmt.Errorf("find datacenter %q: %w", datacenter, err)
	}
	finder.SetDatacenter(dc)

	clusters, err := finder.ClusterComputeResourceList(ctx, path.Join(dc.InventoryPath, "..."))
	if err != nil {
		return nil, fmt.Errorf("list ClusterComputeResources: %w", err)
	}
	clusterRefs := make([]mo.Reference, len(clusters))
	pathByRef := make(map[string]string, len(clusters))
	for i, cl := range clusters {
		clusterRefs[i] = cl
		pathByRef[cl.Reference().String()] = cl.InventoryPath
	}

	tagsClient := tags.NewManager(restC)
	category, err := tagsClient.GetCategory(ctx, *pcc.ZoneTagCategory)
	if err != nil {
		return nil, fmt.Errorf("get tag category %q: %w", *pcc.ZoneTagCategory, err)
	}

	attached, err := tagsClient.GetAttachedTagsOnObjects(ctx, clusterRefs)
	if err != nil {
		return nil, fmt.Errorf("get attached tags on clusters: %w", err)
	}
	wanted := make(map[string]struct{}, len(zones))
	for _, z := range zones {
		wanted[z] = struct{}{}
	}
	result := make(map[string]string, len(zones))
	for _, ct := range attached {
		clusterPath, ok := pathByRef[ct.ObjectID.Reference().String()]
		if !ok {
			continue
		}
		for _, t := range ct.Tags {
			if t.CategoryID != category.ID {
				continue
			}
			if _, want := wanted[t.Name]; !want {
				continue
			}
			result[t.Name] = clusterPath
		}
	}
	return result, nil
}

func networksFromConfig(pcc *v1.VsphereProviderClusterConfiguration) []string {
	if pcc.InternalNetworkNames != nil && len(*pcc.InternalNetworkNames) > 0 {
		return append([]string(nil), (*pcc.InternalNetworkNames)...)
	}
	if pcc.ExternalNetworkNames != nil && len(*pcc.ExternalNetworkNames) > 0 {
		return append([]string(nil), (*pcc.ExternalNetworkNames)...)
	}
	return nil
}

// absFolderPath returns the folder in the "/<datacenter>/vm/<relative>" form CAPV expects.
// providerClusterConfiguration.vmFolderPath wins when set; otherwise fall back to
// providerDiscoveryData.vmFolderPath. Both values are relative to /<datacenter>/vm/.
func absFolderPath(datacenter, discoveredFolder string, cfgFolder *string) string {
	folder := discoveredFolder
	if cfgFolder != nil && *cfgFolder != "" {
		folder = *cfgFolder
	}
	folder = strings.TrimPrefix(folder, "/")
	if folder == "" {
		return ""
	}
	return path.Join("/", datacenter, "vm", folder)
}

// absResourcePoolPath returns "<clusterPath>/Resources/<providerDiscoveryData.resourcePoolPath>",
// which is the absolute form CAPV placementConstraint requires. ResourcePoolPath is a name
// relative to the cluster and produced by the infrastructure step during bootstrap.
func absResourcePoolPath(clusterPath, relativeRP string) string {
	relativeRP = strings.TrimPrefix(relativeRP, "/")
	if relativeRP == "" {
		return path.Join(clusterPath, "Resources")
	}
	return path.Join(clusterPath, "Resources", relativeRP)
}

func datastoreForZone(zone string, datastores []cloudDataV1.VsphereDatastore) string {
	for _, ds := range datastores {
		for _, z := range ds.Zones {
			if z != zone {
				continue
			}
			if ds.InventoryPath != "" {
				return ds.InventoryPath
			}
			return ds.Name
		}
	}
	return ""
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func buildFailureDomain(name, region, regionTagCategory, zone, zoneTagCategory, datacenter, computeCluster, datastore string, networks []string) *unstructured.Unstructured {
	topology := map[string]interface{}{
		"datacenter":     datacenter,
		"computeCluster": computeCluster,
		"datastore":      datastore,
	}
	if len(networks) > 0 {
		nw := make([]interface{}, 0, len(networks))
		for _, n := range networks {
			nw = append(nw, n)
		}
		topology["networks"] = nw
	}
	spec := map[string]interface{}{
		"region": map[string]interface{}{
			"name":        region,
			"type":        "Datacenter",
			"tagCategory": regionTagCategory,
		},
		"zone": map[string]interface{}{
			"name":        zone,
			"type":        "ComputeCluster",
			"tagCategory": zoneTagCategory,
		},
		"topology": topology,
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": fdAPIVersion,
		"kind":       fdKind,
		"metadata": map[string]interface{}{
			"name": name,
			"labels": map[string]interface{}{
				"heritage": "deckhouse",
				"module":   "cloud-provider-vsphere",
			},
		},
		"spec": spec,
	}}
}

func buildDeploymentZone(name, server, failureDomain, folder, resourcePool string) *unstructured.Unstructured {
	placement := map[string]interface{}{}
	if folder != "" {
		placement["folder"] = folder
	}
	if resourcePool != "" {
		placement["resourcePool"] = resourcePool
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": fdAPIVersion,
		"kind":       dzKind,
		"metadata": map[string]interface{}{
			"name": name,
			"labels": map[string]interface{}{
				"heritage": "deckhouse",
				"module":   "cloud-provider-vsphere",
			},
		},
		"spec": map[string]interface{}{
			"server":              server,
			"failureDomain":       failureDomain,
			"controlPlane":        false,
			"placementConstraint": placement,
		},
	}}
}
