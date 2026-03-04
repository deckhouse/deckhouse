/*
Copyright 2024 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	// istioRevLabel is the label on operator-created resources (e.g. ClusterRoleBinding) holding revision like "v1x21x6".
	istioRevLabel = "istio.io/rev"
	// maxIstioMinorVersionForLegacyRBAC limits creation/deletion of operator-created RBAC to Istio > 1.21.
	// For Istio 1.21 and below we do not attempt to create or touch these resources (e.g. skip legacy deletion).
	maxIstioMinorVersionForLegacyRBAC = 21
)

// This hook deletes legacy roles and rolebindings created by operator (not DH) in both scopes
// TODO: Remove this hook after 1.67 (still keep)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "role_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
			FilterFunc: applyRoleFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
		{
			Name:       "rolebinding_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			FilterFunc: applyRoleBindingFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
		{
			Name:       "clusterrole_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyClusterRoleFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
		},
		{
			Name:       "clusterrolebinding_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: applyClusterRoleBindingFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
		},
	},
}, deleteLegacyRBACs)

// roleLikeFilterResult is the result of applyRoleFilter and applyRoleBindingFilter.
// ShouldDelete is false when the object belongs to Istio 1.21 or lower.
type roleLikeFilterResult struct {
	Name         string
	Namespace    string
	ShouldDelete bool
}

// clusterRoleFilterResult is the result of applyClusterRoleFilter.
// ShouldDelete is false when the object belongs to Istio 1.21 or lower.
type clusterRoleFilterResult struct {
	Name         string
	ShouldDelete bool
}

// clusterRoleBindingFilterResult is the result of applyClusterRoleBindingFilter.
// ShouldDelete is false when the object belongs to Istio 1.21 or lower.
type clusterRoleBindingFilterResult struct {
	Name         string
	ShouldDelete bool
}

func getIstioRevisionFromObject(obj *unstructured.Unstructured) string {
	if labels := obj.GetLabels(); labels != nil {
		return labels[istioRevLabel]
	}
	return ""
}

func applyRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	revision := getIstioRevisionFromObject(obj)
	return roleLikeFilterResult{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		ShouldDelete: isIstioVersionAbove121(revision),
	}, nil
}

func applyRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	revision := getIstioRevisionFromObject(obj)
	return roleLikeFilterResult{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		ShouldDelete: isIstioVersionAbove121(revision),
	}, nil
}

func applyClusterRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	revision := getIstioRevisionFromObject(obj)
	return clusterRoleFilterResult{
		Name:         obj.GetName(),
		ShouldDelete: isIstioVersionAbove121(revision),
	}, nil
}

// isIstioVersionAbove121 parses Istio revision (e.g. "v1x21x6") and returns true if version is above 1.21.
// Returns false for 1.21.x and below so we do not attempt to create or delete these RBAC resources for those versions.
func isIstioVersionAbove121(revision string) bool {
	if revision == "" {
		return true // unknown revision: keep previous behaviour (consider for deletion)
	}
	// "v1x21x6" -> ["v1", "21", "6"] or "v1x21" -> ["v1", "21"]
	parts := strings.Split(strings.TrimPrefix(revision, "v"), "x")
	if len(parts) < 2 {
		return true
	}
	major, errMajor := strconv.Atoi(parts[0])
	if errMajor != nil {
		return true
	}
	minor, errMinor := strconv.Atoi(parts[1])
	if errMinor != nil {
		return true
	}
	return major > 1 || (major == 1 && minor > maxIstioMinorVersionForLegacyRBAC)
}

func applyClusterRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	revision := getIstioRevisionFromObject(obj)
	return clusterRoleBindingFilterResult{
		Name:         obj.GetName(),
		ShouldDelete: isIstioVersionAbove121(revision),
	}, nil
}

func deleteLegacyRBACs(_ context.Context, input *go_hook.HookInput) error {
	// remove legacy Roles (only for Istio > 1.21; for 1.21 and below we do not touch)
	for role, err := range sdkobjectpatch.SnapshotIter[roleLikeFilterResult](input.Snapshots.Get("role_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'role_for_delete' snapshot: %w", err)
		}
		if !role.ShouldDelete {
			input.Logger.Info("skip Role %s in %s (Istio 1.21 or below)", role.Name, role.Namespace)
			continue
		}
		input.Logger.Info("remove legacy Role %s in %s namespace", role.Name, role.Namespace)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "Role", role.Namespace, role.Name)
	}

	// remove legacy RoleBindings (only for Istio > 1.21; for 1.21 and below we do not touch)
	for roleBinding, err := range sdkobjectpatch.SnapshotIter[roleLikeFilterResult](input.Snapshots.Get("rolebinding_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'rolebinding_for_delete' snapshot: %w", err)
		}
		if !roleBinding.ShouldDelete {
			input.Logger.Info("skip RoleBinding %s in %s (Istio 1.21 or below)", roleBinding.Name, roleBinding.Namespace)
			continue
		}
		input.Logger.Info("remove legacy RoleBinding %s in %s namespace", roleBinding.Name, roleBinding.Namespace)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", roleBinding.Namespace, roleBinding.Name)
	}

	// remove legacy ClusterRoles (only for Istio > 1.21; for 1.21 and below we do not touch)
	for cr, err := range sdkobjectpatch.SnapshotIter[clusterRoleFilterResult](input.Snapshots.Get("clusterrole_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'clusterrole_for_delete' snapshot: %w", err)
		}
		if !cr.ShouldDelete {
			input.Logger.Info("skip ClusterRole %s (Istio 1.21 or below)", cr.Name)
			continue
		}
		input.Logger.Info("remove legacy ClusterRole %s", cr.Name)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRole", "", cr.Name)
	}

	// remove legacy ClusterRoleBindings (only for Istio > 1.21; for 1.21 and below we do not touch ClusterRoleBinding)
	for crb, err := range sdkobjectpatch.SnapshotIter[clusterRoleBindingFilterResult](input.Snapshots.Get("clusterrolebinding_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'clusterrolebinding_for_delete' snapshot: %w", err)
		}
		if !crb.ShouldDelete {
			input.Logger.Info("skip ClusterRoleBinding %s (Istio 1.21 or below)", crb.Name)
			continue
		}
		input.Logger.Info("remove legacy ClusterRoleBinding %s", crb.Name)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", crb.Name)
	}

	return nil
}
