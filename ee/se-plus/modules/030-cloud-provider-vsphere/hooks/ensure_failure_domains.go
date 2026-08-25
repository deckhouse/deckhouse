/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	v1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// The hook creates VSphereFailureDomain + VSphereDeploymentZone CRs per allowed zone from data
// already published to values by cloud-data-discoverer (Datacenter, Datastores, and — as of the
// new field — ZoneComputeClusterPaths). It does NOT open a vSphere session itself: any vCenter
// round-trip inside a beforeHelm hook blocks Deckhouse's main queue and, on failure, wedges the
// render for the whole module. The reviewer flagged that; the fix is to consume discovery data
// and be event-driven off Kubernetes watches instead.
//
// The pair (FD + DZ) is create-if-not-exists on purpose. CAPV's validating webhook rejects
// updates to VSphereFailureDomain.spec once set, so re-writing here would either be a no-op or
// a hard 422. An admin can hand-tune a FD to fix a wrong topology; the hook must not fight back.
//
// Names go through k8s.io/apimachinery DNS-1123 sanitization (rfc1123SubdomainName). vSphere
// tag names allow spaces and mixed case that Kubernetes object names do not — a raw tag name
// like "E2E Zone 1" would 422 on Create, loop the reconcile forever and, historically, wedge
// beforeHelm. Same trick as discover.go:getStorageClassName.

const (
	fdAPIVersion    = "infrastructure.cluster.x-k8s.io/v1beta1"
	fdKind          = "VSphereFailureDomain"
	dzKind          = "VSphereDeploymentZone"
	fdNamePrefix    = "vsphere-"
	capiClusterNS   = "d8-cloud-instance-manager"
	capiClusterName = "vsphere"
)

var (
	fdGVR      = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspherefailuredomains"}
	dzGVR      = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspheredeploymentzones"}
	clusterGVR = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-vsphere/ensure-failure-domains",
	// OnBeforeHelm guarantees a run on every ModuleRun (initial + on any values change), which
	// covers the case where d8-node-manager-cloud-provider secret does not exist yet at
	// Synchronization time — the Kubernetes binding below would then produce an empty initial
	// snapshot and never fire, and FD/DZ would never be created. This hook does NOT open a
	// vSphere session; it only reads discovery values and Create-if-not-exists two CRs, so it
	// takes single-digit milliseconds and cannot block the main queue the way the previous
	// govmomi-in-beforeHelm variant did (which the reviewer flagged as B3).
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Trigger on the cloud-provider registration secret: it is the vehicle that carries
			// providerClusterConfiguration + providerDiscoveryData into the module's values. Any
			// zone/topology change is published here first, which is exactly when FD/DZ CRs need
			// a refresh.
			Name:       "cloud-provider-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-node-manager-cloud-provider"}},
			// FilterFunc must return a non-nil result: shell-operator dedupes snapshots by
			// filter-result hash, and a nil result on the initial Synchronization + on each
			// Modified event looks identical, so the hook never fires. Returning the object
			// version turns every change into a fresh snapshot entry that triggers the hook.
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return obj.GetResourceVersion(), nil
			},
		},
	},
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
	if pcc.Provider == nil || pcc.Provider.Server == nil {
		return nil
	}

	ddJSON := input.Values.Get("cloudProviderVsphere.internal.providerDiscoveryData").String()
	var dd cloudDataV1.VsphereCloudDiscoveryData
	if ddJSON != "" && ddJSON != "null" {
		if err := json.Unmarshal([]byte(ddJSON), &dd); err != nil {
			return fmt.Errorf("unmarshal providerDiscoveryData: %w", err)
		}
	}
	// Datacenter and ZoneComputeClusterPaths are what makes an absolute topology renderable.
	// Discoverer publishes both; if they are still missing we are ahead of the first discovery
	// tick — no-op and wait for the next event.
	if dd.Datacenter == "" || len(dd.ZoneComputeClusterPaths) == 0 {
		input.Logger.Info("skip FD/DZ ensure: discovery data not populated yet")
		return nil
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	dyn := k8sClient.Dynamic()

	deleting, err := clusterIsDeleting(ctx, dyn)
	if err != nil {
		return fmt.Errorf("check CAPI Cluster deletion state: %w", err)
	}
	if deleting {
		input.Logger.Info("skip FD/DZ ensure: CAPI Cluster is being deleted")
		return nil
	}

	zones := append([]string(nil), (*pcc.Zones)...)
	sort.Strings(zones)

	missing, err := zonesMissingResources(ctx, dyn, zones)
	if err != nil {
		return fmt.Errorf("check existing FD/DZ: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	folder := absFolderPath(dd.Datacenter, dd.VMFolderPath, pcc.VMFolderPath)
	region := strPtrOrEmpty(pcc.Region)
	regionTagCategory := strPtrOrEmpty(pcc.RegionTagCategory)
	zoneTagCategory := strPtrOrEmpty(pcc.ZoneTagCategory)
	server := *pcc.Provider.Server

	for _, zone := range missing {
		clusterPath, ok := dd.ZoneComputeClusterPaths[zone]
		if !ok || clusterPath == "" {
			input.Logger.Warn("skip zone: no compute-cluster path from discovery", "zone", zone)
			continue
		}
		datastore := datastoreForZone(zone, dd.Datastores)
		if datastore == "" {
			input.Logger.Warn("skip zone: no datastore tagged for it", "zone", zone)
			continue
		}
		resourcePool := absResourcePoolPath(clusterPath, dd.ResourcePoolPath)

		fdName := fdNamePrefix + rfc1123SubdomainName(zone)
		dzName := rfc1123SubdomainName(zone)

		fd := buildFailureDomain(fdName, region, regionTagCategory, zone, zoneTagCategory, dd.Datacenter, clusterPath, datastore)
		dz := buildDeploymentZone(dzName, server, fdName, folder, resourcePool)
		input.PatchCollector.CreateIfNotExists(fd)
		input.PatchCollector.CreateIfNotExists(dz)
		input.Logger.Info("created VSphereFailureDomain and VSphereDeploymentZone",
			"zone", zone, "fdName", fdName, "dzName", dzName,
			"computeCluster", clusterPath, "datastore", datastore)
	}
	return nil
}

// clusterIsDeleting targets exactly the CAPI Cluster this module owns
// (d8-cloud-instance-manager/vsphere). A cluster-wide List would treat any unrelated or leaked
// Cluster in Deleting state as a signal to stop reconciling FD/DZ, which wedges the module.
func clusterIsDeleting(ctx context.Context, dyn dynamic.Interface) (bool, error) {
	obj, err := dyn.Resource(clusterGVR).Namespace(capiClusterNS).Get(ctx, capiClusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, fmt.Errorf("get CAPI Cluster %s/%s: %w", capiClusterNS, capiClusterName, err)
	}
	return obj.GetDeletionTimestamp() != nil, nil
}

func zonesMissingResources(ctx context.Context, dyn dynamic.Interface, zones []string) ([]string, error) {
	fdList, err := dyn.Resource(fdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if meta.IsNoMatchError(err) {
			return append([]string(nil), zones...), nil
		}
		return nil, fmt.Errorf("list VSphereFailureDomains: %w", err)
	}
	dzList, err := dyn.Resource(dzGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if meta.IsNoMatchError(err) {
			return append([]string(nil), zones...), nil
		}
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
		_, hasFD := fdNames[fdNamePrefix+rfc1123SubdomainName(zone)]
		_, hasDZ := dzNames[rfc1123SubdomainName(zone)]
		if !hasFD || !hasDZ {
			missing = append(missing, zone)
		}
	}
	return missing, nil
}

// rfc1123SubdomainName maps a vSphere tag name to a valid Kubernetes DNS-1123 subdomain:
// lowercase alphanumerics, '-' and '.', starting and ending with an alphanumeric character.
// Mirrors discover.go:getStorageClassName. Without this, a tag like "E2E Zone 1" would fail
// the object-name validator at Create with 422 and loop the reconcile forever.
//
// capi/template.yaml duplicates this sanitization in sprig (`lower | replace " " "-" |
// regexReplaceAll "[^a-z0-9.-]" ""` + `trimAll "-."`), because Machine.Spec.FailureDomain must
// match VSphereDeploymentZone.metadata.name for CAPV's override to fire. Keeping the two
// copies in sync is enforced by ensure_failure_domains_sanitize_parity_test.go — it extracts
// the literal from template.yaml, runs it through the same text/template sandbox
// node-controller uses at render time, and asserts equality with this function on a golden
// set of inputs.
func rfc1123SubdomainName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		} else if r == ' ' {
			return '-'
		}
		return rune(-1)
	}
	return strings.Trim(strings.Map(mapFn, value), "-.")
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

// datastoreForZone picks a datastore deterministically: sort every datastore that carries the
// zone tag by InventoryPath (fall back to Name), then take the first. Deterministic order matters
// because FailureDomain.topology.datastore is immutable via the CAPV webhook.
func datastoreForZone(zone string, datastores []cloudDataV1.VsphereDatastore) string {
	var matches []string
	for _, ds := range datastores {
		for _, z := range ds.Zones {
			if z != zone {
				continue
			}
			ref := ds.InventoryPath
			if ref == "" {
				ref = ds.Name
			}
			if ref != "" {
				matches = append(matches, ref)
			}
			break
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func buildFailureDomain(name, region, regionTagCategory, zone, zoneTagCategory, datacenter, computeCluster, datastore string) *unstructured.Unstructured {
	topology := map[string]interface{}{
		"datacenter":     datacenter,
		"computeCluster": computeCluster,
		"datastore":      datastore,
	}
	// autoConfigure is a *bool in CAPV and reconcileInfraFailureDomain dereferences it
	// unconditionally (vspheredeploymentzone_controller_domain.go). The CRD schema has no default
	// and Deckhouse does not ship the upstream MutatingWebhookConfiguration for CAPV (it caused a
	// teardown deadlock — apiserver could not reach the same-cluster webhook once cilium started
	// removing endpoints and no VSphereVM finalizer would ever be patched off). So we set false
	// explicitly here: the tags are already attached by the admin at cluster bootstrap. An FD
	// authored outside this hook without autoConfigure set would nil-deref the DZ reconciler —
	// document that in the module docs, do not paper over it with a webhook we cannot rely on
	// during teardown.
	spec := map[string]interface{}{
		"region": map[string]interface{}{
			"name":          region,
			"type":          "Datacenter",
			"tagCategory":   regionTagCategory,
			"autoConfigure": false,
		},
		"zone": map[string]interface{}{
			"name":          zone,
			"type":          "ComputeCluster",
			"tagCategory":   zoneTagCategory,
			"autoConfigure": false,
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
