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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	v1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-vsphere/hooks/internal/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// CAPV attaches tags by URN ID only (unlike MCM, which creates tags by name at
// VM creation). This hook mirrors MCM's deckhouse-cluster-name / deckhouse-node-role
// categories, ensures the tags exist, and publishes their URNs into module values
// so registration.yaml can pass them to VSphereMachineTemplate.

const (
	clusterNameTagCategory = "deckhouse-cluster-name"
	nodeRoleTagCategory    = "deckhouse-node-role"
)

var nodeGroupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/cloud-provider-vsphere/ensure-capi-vm-tags",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 25},
}, dependency.WithExternalDependencies(ensureCAPIVMTags))

func ensureCAPIVMTags(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	pccJSON := input.Values.Get("cloudProviderVsphere.internal.providerClusterConfiguration").String()
	if pccJSON == "" || pccJSON == "null" {
		return nil
	}
	var pcc v1.VsphereProviderClusterConfiguration
	if err := json.Unmarshal([]byte(pccJSON), &pcc); err != nil {
		return fmt.Errorf("unmarshal providerClusterConfiguration: %w", err)
	}
	if pcc.Provider == nil || pcc.Provider.Server == nil || pcc.Provider.Username == nil || pcc.Provider.Password == nil {
		return nil
	}

	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()
	if clusterUUID == "" {
		input.Logger.Warn("global.discovery.clusterUUID is empty, skipping CAPI VM tag ensure")
		return nil
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	roleKeys, err := listCloudEphemeralNodeRoleKeys(ctx, k8sClient.Dynamic())
	if err != nil {
		return fmt.Errorf("list NodeGroups for node-role tags: %w", err)
	}

	tagger, err := newVsphereTagger(ctx, &pcc)
	if err != nil {
		return fmt.Errorf("open vsphere tag session: %w", err)
	}
	defer tagger.Close(ctx)

	clusterTagID, err := tagger.EnsureTag(ctx, clusterNameTagCategory, clusterUUID)
	if err != nil {
		return fmt.Errorf("ensure cluster-name tag %q: %w", clusterUUID, err)
	}

	nodeRoleTagIDs := make(map[string]string, len(roleKeys))
	for _, key := range roleKeys {
		id, err := tagger.EnsureTag(ctx, nodeRoleTagCategory, key)
		if err != nil {
			return fmt.Errorf("ensure node-role tag %q: %w", key, err)
		}
		nodeRoleTagIDs[key] = id
	}

	input.Values.Set("cloudProviderVsphere.internal.capiClusterTagID", clusterTagID)
	input.Values.Set("cloudProviderVsphere.internal.capiNodeRoleTagIDs", nodeRoleTagIDs)
	return nil
}

func listCloudEphemeralNodeRoleKeys(ctx context.Context, dyn dynamic.Interface) ([]string, error) {
	list, err := dyn.Resource(nodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list NodeGroups: %w", err)
	}
	return nodeRoleKeysFromNodeGroups(list.Items), nil
}

func nodeRoleKeysFromNodeGroups(items []unstructured.Unstructured) []string {
	var keys []string
	seen := map[string]struct{}{}
	for _, ng := range items {
		nodeType, _, _ := unstructured.NestedString(ng.Object, "spec", "nodeType")
		if nodeType != "CloudEphemeral" && nodeType != "Cloud" {
			continue
		}
		zones, _, _ := unstructured.NestedStringSlice(ng.Object, "spec", "cloudInstances", "zones")
		name := ng.GetName()
		for _, zone := range zones {
			key := name + "-" + zone
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

type vsphereTagger struct {
	restC   *rest.Client
	tags    *tags.Manager
	govmomi *govmomi.Client
}

func newVsphereTagger(ctx context.Context, pcc *v1.VsphereProviderClusterConfiguration) (*vsphereTagger, error) {
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
	restC := rest.NewClient(c.Client)
	if err := restC.Login(ctx, u.User); err != nil {
		_ = c.Logout(ctx)
		return nil, fmt.Errorf("vsphere rest login: %w", err)
	}
	return &vsphereTagger{restC: restC, tags: tags.NewManager(restC), govmomi: c}, nil
}

func (t *vsphereTagger) Close(ctx context.Context) {
	_ = t.restC.Logout(ctx)
	_ = t.govmomi.Logout(ctx)
}

func (t *vsphereTagger) EnsureTag(ctx context.Context, categoryName, tagName string) (string, error) {
	categoryID, err := t.ensureCategory(ctx, categoryName)
	if err != nil {
		return "", err
	}
	existing, err := t.tags.GetTagForCategory(ctx, tagName, categoryName)
	if err == nil && existing != nil && existing.ID != "" {
		return existing.ID, nil
	}
	id, err := t.tags.CreateTag(ctx, &tags.Tag{
		Name:        tagName,
		CategoryID:  categoryID,
		Description: "Created by Deckhouse cloud-provider-vsphere for CAPV",
	})
	if err != nil {
		existing, getErr := t.tags.GetTagForCategory(ctx, tagName, categoryName)
		if getErr == nil && existing != nil && existing.ID != "" {
			return existing.ID, nil
		}
		return "", fmt.Errorf("create tag %q in category %q: %w", tagName, categoryName, err)
	}
	return id, nil
}

func (t *vsphereTagger) ensureCategory(ctx context.Context, name string) (string, error) {
	cat, err := t.tags.GetCategory(ctx, name)
	if err == nil && cat != nil && cat.ID != "" {
		return cat.ID, nil
	}
	id, err := t.tags.CreateCategory(ctx, &tags.Category{
		Name:            name,
		Description:     "Created by Deckhouse cloud-provider-vsphere for CAPV",
		Cardinality:     "MULTIPLE",
		AssociableTypes: []string{"VirtualMachine"},
	})
	if err != nil {
		cat, getErr := t.tags.GetCategory(ctx, name)
		if getErr == nil && cat != nil && cat.ID != "" {
			return cat.ID, nil
		}
		return "", fmt.Errorf("create category %q: %w", name, err)
	}
	return id, nil
}
