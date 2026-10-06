/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Discovers the bindings that name a capability of the experimental role model directly: every
// ClusterRoleBinding and RoleBinding without heritage: deckhouse whose roleRef names one of the
// d8:manage:permission:* and d8:use:capability:* capabilities of the modules shipped with Deckhouse
// or of user-authz itself. The new role model renames these capabilities without compatibility
// aliases (legacyCapabilityRenames), so after the update such a binding names a ClusterRole that no
// longer exists and grants nothing.
//
// Each binding drives the D8UserAuthzLegacyRBACv2CapabilityBindingFound alert, which names the new
// name of the capability. A module installed from a module source renames its capabilities in a
// release of its own, so a binding to one of them is not counted.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	legacyCapabilityClusterRoleBindingsSnapshot = "legacy_capability_cluster_role_bindings"
	legacyCapabilityRoleBindingsSnapshot        = "legacy_capability_role_bindings"

	// legacyCapabilityBindingMetric drives the D8UserAuthzLegacyRBACv2CapabilityBindingFound alert:
	// one time series per binding, labelled with the binding, the capability it names and the new
	// name of that capability.
	legacyCapabilityBindingMetric = "d8_rbacv2_legacy_capability_binding"
)

// systemCapabilityModules are the modules shipped with Deckhouse whose
// d8:manage:permission:module:<module>:view and :edit capabilities the new role model renames to
// d8:system-capability:<module>:view and :edit.
var systemCapabilityModules = []string{
	"admission-policy-engine",
	"cert-manager",
	"chrony",
	"cilium-hubble",
	"cloud-provider-aws",
	"cloud-provider-azure",
	"cloud-provider-dvp",
	"cloud-provider-dynamix",
	"cloud-provider-gcp",
	"cloud-provider-huaweicloud",
	"cloud-provider-openstack",
	"cloud-provider-vcd",
	"cloud-provider-vsphere",
	"cloud-provider-yandex",
	"cloud-provider-zvirt",
	"cni-cilium",
	"cni-flannel",
	"cni-simple-bridge",
	"control-plane-manager",
	"csi-vsphere",
	"deckhouse",
	"descheduler",
	"istio",
	"keepalived",
	"kube-dns",
	"kube-proxy",
	"local-path-provisioner",
	"metallb",
	"multitenancy-manager",
	"network-gateway",
	"network-policy-engine",
	"node-local-dns",
	"node-manager",
	"okmeter",
	"openvpn",
	"secret-copier",
	"service-with-healthchecks",
	"terraform-manager",
	"user-authn",
	"user-authz",
	"vertical-pod-autoscaler",
}

// namespaceCapabilityActions are the actions of the d8:use:capability:module:<module>:<action>
// capabilities of the modules shipped with Deckhouse, which the new role model renames to
// d8:namespace-capability:<module>:<action>.
var namespaceCapabilityActions = map[string][]string{
	"cert-manager":              {"view", "edit"},
	"cni-cilium":                {"view", "edit"},
	"deckhouse":                 {"view", "edit"},
	"multitenancy-manager":      {"view"},
	"service-with-healthchecks": {"view", "edit"},
	"user-authn":                {"view", "edit"},
	"user-authz":                {"view", "edit"},
}

// subsystemCapabilitySubsystems maps the subsystems of the
// d8:manage:permission:subsystem:<subsystem>:<action> capabilities of user-authz to the subsystems
// of their new names, d8:subsystem-capability:<subsystem>:<action>. The new subsystem list merges
// kubernetes into cluster and renames networking to network.
var subsystemCapabilitySubsystems = map[string]string{
	"kubernetes": "cluster",
	"networking": "network",
	"security":   "security",
	"storage":    "storage",
}

// subsystemCapabilityActions are the actions of each subsystem in subsystemCapabilitySubsystems.
var subsystemCapabilityActions = []string{"manage_resources", "view_resources"}

// kubernetesUseCapabilityActions are the actions of the d8:use:capability:kubernetes:<action>
// capabilities of user-authz, which the new role model renames to
// d8:namespace-capability:kubernetes:<action>.
var kubernetesUseCapabilityActions = []string{
	"access_terminal",
	"delete_pods",
	"forward_port",
	"manage_limits",
	"manage_quotas",
	"manage_resources",
	"manage_security",
	"proxy_resources",
	"scale_controllers",
	"view_logs",
	"view_metrics",
	"view_resources",
	"view_secrets",
}

// legacyCapabilityRenames maps the name of every capability above to its name in the new role model.
var legacyCapabilityRenames = buildLegacyCapabilityRenames()

func buildLegacyCapabilityRenames() map[string]string {
	renames := make(map[string]string)
	for _, module := range systemCapabilityModules {
		for _, action := range []string{"view", "edit"} {
			renames["d8:manage:permission:module:"+module+":"+action] = "d8:system-capability:" + module + ":" + action
		}
	}
	for module, actions := range namespaceCapabilityActions {
		for _, action := range actions {
			renames["d8:use:capability:module:"+module+":"+action] = "d8:namespace-capability:" + module + ":" + action
		}
	}
	for subsystem, newSubsystem := range subsystemCapabilitySubsystems {
		for _, action := range subsystemCapabilityActions {
			renames["d8:manage:permission:subsystem:"+subsystem+":"+action] = "d8:subsystem-capability:" + newSubsystem + ":" + action
		}
	}
	for _, action := range kubernetesUseCapabilityActions {
		renames["d8:use:capability:kubernetes:"+action] = "d8:namespace-capability:kubernetes:" + action
	}
	return renames
}

// notDeckhouseSelector leaves out the bindings that Deckhouse creates itself, such as the
// RoleBindings it projects into the namespaces of the modules.
var notDeckhouseSelector = &metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key:      "heritage",
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{"deckhouse"},
	}},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("legacy_capability_bindings"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:          legacyCapabilityClusterRoleBindingsSnapshot,
			ApiVersion:    "rbac.authorization.k8s.io/v1",
			Kind:          "ClusterRoleBinding",
			LabelSelector: notDeckhouseSelector,
			FilterFunc:    applyLegacyCapabilityClusterRoleBindingFilter,
		},
		{
			Name:          legacyCapabilityRoleBindingsSnapshot,
			ApiVersion:    "rbac.authorization.k8s.io/v1",
			Kind:          "RoleBinding",
			LabelSelector: notDeckhouseSelector,
			FilterFunc:    applyLegacyCapabilityRoleBindingFilter,
		},
	},
}, handleLegacyCapabilityBindings)

// legacyCapabilityBinding is what the snapshots keep of a binding to a renamed capability.
type legacyCapabilityBinding struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	RoleName  string `json:"roleName"`
}

// applyLegacyCapabilityClusterRoleBindingFilter keeps only the ClusterRoleBindings to a renamed
// capability, so the snapshot never holds the rest of the cluster's bindings.
func applyLegacyCapabilityClusterRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.ClusterRoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if !namesRenamedCapability(binding.RoleRef) {
		return nil, nil
	}
	return legacyCapabilityBinding{Kind: "ClusterRoleBinding", Name: binding.Name, RoleName: binding.RoleRef.Name}, nil
}

// applyLegacyCapabilityRoleBindingFilter keeps only the RoleBindings to a renamed capability.
func applyLegacyCapabilityRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.RoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if !namesRenamedCapability(binding.RoleRef) {
		return nil, nil
	}
	return legacyCapabilityBinding{
		Kind:      "RoleBinding",
		Name:      binding.Name,
		Namespace: binding.Namespace,
		RoleName:  binding.RoleRef.Name,
	}, nil
}

// namesRenamedCapability reports whether a roleRef names a ClusterRole that the new role model
// renames without an alias.
func namesRenamedCapability(roleRef rbacv1.RoleRef) bool {
	if roleRef.Kind != "ClusterRole" {
		return false
	}
	_, renamed := legacyCapabilityRenames[roleRef.Name]
	return renamed
}

// handleLegacyCapabilityBindings publishes one alert metric per binding on every synchronization or
// event, so the alert resolves as soon as a binding is deleted.
func handleLegacyCapabilityBindings(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(legacyCapabilityBindingMetric)
	for _, snapshot := range []string{legacyCapabilityClusterRoleBindingsSnapshot, legacyCapabilityRoleBindingsSnapshot} {
		for binding, err := range sdkobjectpatch.SnapshotIter[legacyCapabilityBinding](input.Snapshots.Get(snapshot)) {
			if err != nil {
				return fmt.Errorf("failed to iterate over '%s' snapshot: %w", snapshot, err)
			}
			input.MetricsCollector.Set(legacyCapabilityBindingMetric, 1,
				map[string]string{
					"binding_kind": binding.Kind,
					"binding_name": binding.Name,
					"namespace":    binding.Namespace,
					"role_name":    binding.RoleName,
					"replaced_by":  legacyCapabilityRenames[binding.RoleName],
				},
				metrics.WithGroup(legacyCapabilityBindingMetric))
		}
	}
	return nil
}
