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

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	v1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// The hook maintains two shapes of VSphereDeploymentZone (DZ):
//
//  1. Zone-baseline: one DZ per allowed zone from providerClusterConfiguration.zones, linked
//     to one VSphereFailureDomain (FD) per zone. resourcePool comes from
//     providerDiscoveryData.resourcePoolPath — the module-wide default. FD.spec is immutable
//     (CAPV webhook), so these are create-if-not-exists.
//
//  2. NG override: an extra DZ per (zone × NodeGroup) whose VsphereInstanceClass carries
//     spec.resourcePool. The extra DZ links to the same FD as the baseline, but its
//     placementConstraint.resourcePool is the InstanceClass value. capi/template.yaml
//     encodes the choice in Machine.Spec.FailureDomain — plain sanZone for baseline,
//     compound "sanZone-sanNG" for override. CAPV's overrideFunc reads the DZ CAPV picks
//     up via that string and applies PlacementConstraint.ResourcePool on VM clone.
//     Datastore is not part of this scheme: CAPV reads datastore from
//     VSphereFailureDomain.spec.topology.datastore, which is one per zone and immutable
//     via the FD webhook — a per-NG datastore override isn't reachable without recreating
//     the FD, so InstanceClass.spec.datastore stays module-wide.
//
// Override DZs are pure Kubernetes objects — CAPV does not attach finalizers to them, so
// delete is synchronous. When an operator removes spec.resourcePool from the InstanceClass
// (or deletes the NodeGroup), the hook deletes the extra DZ. Existing VMs stay put
// (VSphereVM.spec is immutable). New VMs the MachineSet clones after the deletion fall
// back to the baseline DZ, which matches the operator's intent — they explicitly turned
// off the override.
//
// The hook does not open a vSphere session; it only reads discovery values + snapshots
// and diffs against existing DZs. Runs in the module queue, event-driven off three
// bindings: the registration Secret (existing plumbing), VsphereInstanceClass, and
// NodeGroup.
//
// Names go through DNS-1123 sanitization (rfc1123SubdomainName). The same sanitization
// is duplicated in capi/template.yaml on the sprig side (Machine.Spec.FailureDomain must
// match DZ.name for CAPV to find the DZ). Drift between the two implementations fails
// hooks/ensure_failure_domains_sanitize_parity_test.go on the next CI run.

const (
	fdAPIVersion    = "infrastructure.cluster.x-k8s.io/v1beta1"
	fdKind          = "VSphereFailureDomain"
	dzKind          = "VSphereDeploymentZone"
	fdNamePrefix    = "vsphere-"
	capiClusterNS   = "d8-cloud-instance-manager"
	capiClusterName = "vsphere"

	// dzTypeLabel differentiates DZs the hook manages: "base" (one per zone, immutable
	// resourcePool from discovery default) and "override" (one per NG that carries a
	// resourcePool override). Only DZs with dzTypeLabel="override" are eligible for
	// deletion in the reconcile diff — a stray label mismatch on a base DZ must never
	// cause a base DZ to be reaped.
	dzTypeLabel     = "cloud-provider-vsphere.deckhouse.io/dz-type"
	dzTypeBase      = "base"
	dzTypeOverride  = "override"
	dzNodeGroupLabel = "cloud-provider-vsphere.deckhouse.io/node-group"

	instanceClassKind = "VsphereInstanceClass"
)

var (
	fdGVR      = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspherefailuredomains"}
	dzGVR      = schema.GroupVersionResource{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta1", Resource: "vspheredeploymentzones"}
	clusterGVR = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "clusters"}
)

// instanceClassSnapshot is what the VsphereInstanceClass FilterFunc returns per object.
// Only the InstanceClass name and its resourcePool matter for reconciler logic — an empty
// resourcePool means "no override" and the InstanceClass falls back to the module-wide
// baseline DZ.
type instanceClassSnapshot struct {
	Name         string `json:"name"`
	ResourcePool string `json:"resourcePool"`
}

// nodeGroupSnapshot is what the NodeGroup FilterFunc returns per object. Fields are the
// minimum needed to decide (a) whether the NG is a vsphere CAPI NG, (b) which
// InstanceClass to look up, and (c) which zones the NG spans. An NG with empty
// spec.cloudInstances.zones defaults to all providerClusterConfiguration.zones at
// reconcile time — the empty list is preserved here so that pcc changes propagate.
type nodeGroupSnapshot struct {
	Name              string   `json:"name"`
	InstanceClassKind string   `json:"instanceClassKind"`
	InstanceClassName string   `json:"instanceClassName"`
	Zones             []string `json:"zones"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-vsphere/ensure-failure-domains",
	// OnBeforeHelm guarantees a run on every ModuleRun (initial + on any values change), which
	// covers the case where none of the Kubernetes bindings has produced a snapshot yet at
	// Synchronization time — the bindings below would produce empty initial snapshots and the
	// hook would never fire, and the baseline DZs would never be created. The reconciler does
	// NOT open a vSphere session; it only reads discovery values, snapshots and Kubernetes
	// state, so it takes single-digit milliseconds and cannot block the main queue the way
	// the previous govmomi-in-beforeHelm variant did (which the reviewer flagged as B3).
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Trigger on the cloud-provider registration secret: it is the vehicle that carries
			// providerClusterConfiguration + providerDiscoveryData into the module's values. Any
			// zone/topology change is published here first, which is exactly when the baseline
			// DZs need a refresh.
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
		{
			// VsphereInstanceClass provides the resourcePool override. The hook re-runs whenever
			// an InstanceClass is created, updated (spec.resourcePool set/changed/cleared) or
			// deleted. FilterFunc surfaces only the name and resourcePool — the rest of the
			// InstanceClass spec is irrelevant to this reconciler.
			Name:       "vsphere-instance-classes",
			ApiVersion: "deckhouse.io/v1",
			Kind:       instanceClassKind,
			FilterFunc: filterVsphereInstanceClass,
			// Silence the "no ExecuteHookOnEvent, treating as OnBeforeHelm-only" behavior: we
			// need actual events, not just startup snapshot.
			ExecuteHookOnEvents:          ptrTrue(),
			ExecuteHookOnSynchronization: ptrTrue(),
		},
		{
			// NodeGroup identifies which InstanceClass a NG uses and (optionally) constrains
			// zones. Only NGs that reference a VsphereInstanceClass are relevant, but filtering
			// server-side isn't possible (kind is in spec, not labels) — the reconciler filters
			// the snapshot instead.
			Name:                         "node-groups",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			FilterFunc:                   filterNodeGroup,
			ExecuteHookOnEvents:          ptrTrue(),
			ExecuteHookOnSynchronization: ptrTrue(),
		},
	},
}, dependency.WithExternalDependencies(ensureFailureDomains))

func ptrTrue() *bool { v := true; return &v }

func filterVsphereInstanceClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	rp, _, err := unstructured.NestedString(obj.Object, "spec", "resourcePool")
	if err != nil {
		return nil, fmt.Errorf("read spec.resourcePool of VsphereInstanceClass %q: %w", obj.GetName(), err)
	}
	return instanceClassSnapshot{
		Name:         obj.GetName(),
		ResourcePool: rp,
	}, nil
}

func filterNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, err := unstructured.NestedString(obj.Object, "spec", "cloudInstances", "classReference", "kind")
	if err != nil {
		return nil, fmt.Errorf("read spec.cloudInstances.classReference.kind of NodeGroup %q: %w", obj.GetName(), err)
	}
	icName, _, err := unstructured.NestedString(obj.Object, "spec", "cloudInstances", "classReference", "name")
	if err != nil {
		return nil, fmt.Errorf("read spec.cloudInstances.classReference.name of NodeGroup %q: %w", obj.GetName(), err)
	}
	zones, _, err := unstructured.NestedStringSlice(obj.Object, "spec", "cloudInstances", "zones")
	if err != nil {
		return nil, fmt.Errorf("read spec.cloudInstances.zones of NodeGroup %q: %w", obj.GetName(), err)
	}
	return nodeGroupSnapshot{
		Name:              obj.GetName(),
		InstanceClassKind: kind,
		InstanceClassName: icName,
		Zones:             zones,
	}, nil
}

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

	// Baseline pass: create the per-zone FD + DZ if absent. FD is immutable (CAPV webhook)
	// so this is always CreateIfNotExists — mutation attempts would 422. Base DZ is also
	// created once with the module-wide resourcePool; changes to that default do not
	// propagate to existing base DZs, which matches the current documented behavior.
	missing, err := zonesMissingBaselineResources(ctx, dyn, zones)
	if err != nil {
		return fmt.Errorf("list baseline FD/DZ: %w", err)
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
		baselineRP := absResourcePoolPath(clusterPath, dd.ResourcePoolPath)

		fdName := fdNamePrefix + rfc1123SubdomainName(zone)
		dzName := rfc1123SubdomainName(zone)

		fd := buildFailureDomain(fdName, region, regionTagCategory, zone, zoneTagCategory, dd.Datacenter, clusterPath, datastore)
		dz := buildDeploymentZone(dzName, server, fdName, folder, baselineRP, dzTypeBase, "")
		input.PatchCollector.CreateIfNotExists(fd)
		input.PatchCollector.CreateIfNotExists(dz)
		input.Logger.Info("created baseline VSphereFailureDomain and VSphereDeploymentZone",
			"zone", zone, "fdName", fdName, "dzName", dzName,
			"computeCluster", clusterPath, "datastore", datastore)
	}

	// Override pass: consume the InstanceClass + NodeGroup snapshots, compute the desired
	// set of override DZs, then reconcile (create/update newly needed, delete no-longer
	// needed). Only DZs with dzTypeLabel=override are touched — a base DZ never falls into
	// the deletion diff.
	overrides, err := desiredOverrideDZs(input.Snapshots, *pcc.Zones)
	if err != nil {
		return fmt.Errorf("compute desired override DZs: %w", err)
	}
	if err := reconcileOverrideDZs(ctx, input, dyn, server, folder, overrides); err != nil {
		return fmt.Errorf("reconcile override DZs: %w", err)
	}

	return nil
}

// desiredOverrideDZ describes one override DZ the reconciler wants to exist. Zone and
// NodeGroup are the identity (a DZ name is derived from them via sanitization); resourcePool
// is the value written into placementConstraint. The name field is precomputed by
// dzNameOverride to keep sanitization consolidated.
type desiredOverrideDZ struct {
	Name         string
	Zone         string
	NodeGroup    string
	ResourcePool string
	FDName       string
}

// desiredOverrideDZs builds the target set of override DZs from the InstanceClass and
// NodeGroup snapshots. A NodeGroup contributes one override DZ per zone iff:
//   - it references an InstanceClass of kind VsphereInstanceClass;
//   - the referenced InstanceClass exists in the snapshot;
//   - the InstanceClass has a non-empty spec.resourcePool.
//
// pccZones is the fallback zone list for a NodeGroup that leaves spec.cloudInstances.zones
// empty. Result is keyed by DZ name for O(1) diff.
func desiredOverrideDZs(snaps sdkpkg.Snapshots, pccZones []string) (map[string]desiredOverrideDZ, error) {
	ics, err := sdkobjectpatch.UnmarshalToStruct[instanceClassSnapshot](snaps, "vsphere-instance-classes")
	if err != nil {
		return nil, fmt.Errorf("unmarshal vsphere-instance-classes snapshot: %w", err)
	}
	icByName := map[string]string{} // ic name → resourcePool
	for _, ic := range ics {
		if ic.ResourcePool == "" {
			continue
		}
		icByName[ic.Name] = ic.ResourcePool
	}

	ngs, err := sdkobjectpatch.UnmarshalToStruct[nodeGroupSnapshot](snaps, "node-groups")
	if err != nil {
		return nil, fmt.Errorf("unmarshal node-groups snapshot: %w", err)
	}
	desired := map[string]desiredOverrideDZ{}
	for _, ng := range ngs {
		if ng.InstanceClassKind != instanceClassKind {
			continue
		}
		rp, has := icByName[ng.InstanceClassName]
		if !has {
			continue
		}
		zones := ng.Zones
		if len(zones) == 0 {
			zones = pccZones
		}
		for _, z := range zones {
			dz := desiredOverrideDZ{
				Name:         dzNameOverride(z, ng.Name),
				Zone:         z,
				NodeGroup:    ng.Name,
				ResourcePool: rp,
				FDName:       fdNamePrefix + rfc1123SubdomainName(z),
			}
			desired[dz.Name] = dz
		}
	}
	return desired, nil
}

func dzNameOverride(zone, ngName string) string {
	return rfc1123SubdomainName(zone) + "-" + rfc1123SubdomainName(ngName)
}

// reconcileOverrideDZs applies the desired set: creates or updates each desired DZ, then
// deletes any DZ carrying dzTypeLabel=override that isn't in the desired set. Updates go
// through CreateOrUpdate because VSphereDeploymentZone.spec has no ValidateUpdate webhook
// in CAPV v1.15.3 — placementConstraint is freely mutable. Delete is unconditional (no
// finalizer coordination): CAPV does not attach finalizers to DZ, and once the DZ is gone
// the next reconcile of Machines that referenced it falls back to baseline placement,
// which matches the operator's intent when they removed the override.
func reconcileOverrideDZs(
	ctx context.Context,
	input *go_hook.HookInput,
	dyn dynamic.Interface,
	server, folder string,
	desired map[string]desiredOverrideDZ,
) error {
	existing, err := listExistingOverrideDZNames(ctx, dyn)
	if err != nil {
		return err
	}

	for _, dz := range desired {
		obj := buildDeploymentZone(dz.Name, server, dz.FDName, folder, dz.ResourcePool, dzTypeOverride, dz.NodeGroup)
		input.PatchCollector.CreateOrUpdate(obj)
		if _, present := existing[dz.Name]; !present {
			input.Logger.Info("created override VSphereDeploymentZone",
				"dzName", dz.Name, "zone", dz.Zone, "nodeGroup", dz.NodeGroup, "resourcePool", dz.ResourcePool)
		}
	}

	for name := range existing {
		if _, keep := desired[name]; keep {
			continue
		}
		input.PatchCollector.Delete(fdAPIVersion, dzKind, "", name)
		input.Logger.Info("deleted orphaned override VSphereDeploymentZone", "dzName", name)
	}
	return nil
}

// listExistingOverrideDZNames returns the names of every DZ this hook has previously
// created as an override (dzTypeLabel=override). Base DZs are excluded intentionally.
func listExistingOverrideDZNames(ctx context.Context, dyn dynamic.Interface) (map[string]struct{}, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: dzTypeLabel + "=" + dzTypeOverride,
	}
	list, err := dyn.Resource(dzGVR).List(ctx, listOpts)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("list override VSphereDeploymentZones: %w", err)
	}
	out := make(map[string]struct{}, len(list.Items))
	for _, o := range list.Items {
		out[o.GetName()] = struct{}{}
	}
	return out, nil
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

// zonesMissingBaselineResources returns the zones whose baseline FD or baseline DZ does not
// yet exist. Override DZs are ignored here — they are reconciled separately, and a missing
// baseline is what needs a create-if-not-exists pass.
func zonesMissingBaselineResources(ctx context.Context, dyn dynamic.Interface, zones []string) ([]string, error) {
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
	baselineDZNames := make(map[string]struct{}, len(dzList.Items))
	for _, o := range dzList.Items {
		// Objects created before dzTypeLabel existed have no label — count them as base
		// so that an upgrade from the pre-label hook does not spuriously recreate them.
		labels := o.GetLabels()
		if t, ok := labels[dzTypeLabel]; ok && t != dzTypeBase {
			continue
		}
		baselineDZNames[o.GetName()] = struct{}{}
	}
	var missing []string
	for _, zone := range zones {
		_, hasFD := fdNames[fdNamePrefix+rfc1123SubdomainName(zone)]
		_, hasDZ := baselineDZNames[rfc1123SubdomainName(zone)]
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

// buildDeploymentZone constructs a VSphereDeploymentZone object. dzType is either "base"
// (per-zone default) or "override" (per (zone × NG) override); the label is what
// reconcileOverrideDZs uses to safely list-and-delete only override DZs without ever
// touching a base DZ. When dzType is "override", nodeGroup carries the NG identity for
// operator debugging (kubectl get vspheredeploymentzones --show-labels).
func buildDeploymentZone(name, server, failureDomain, folder, resourcePool, dzType, nodeGroup string) *unstructured.Unstructured {
	placement := map[string]interface{}{}
	if folder != "" {
		placement["folder"] = folder
	}
	if resourcePool != "" {
		placement["resourcePool"] = resourcePool
	}
	labels := map[string]interface{}{
		"heritage":  "deckhouse",
		"module":    "cloud-provider-vsphere",
		dzTypeLabel: dzType,
	}
	if nodeGroup != "" {
		labels[dzNodeGroupLabel] = nodeGroup
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": fdAPIVersion,
		"kind":       dzKind,
		"metadata": map[string]interface{}{
			"name":   name,
			"labels": labels,
		},
		"spec": map[string]interface{}{
			"server":              server,
			"failureDomain":       failureDomain,
			"controlPlane":        false,
			"placementConstraint": placement,
		},
	}}
}
