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

// reconcile_kubeadm_cluster_admins_binding is the SINGLE SOURCE OF TRUTH for ClusterRoleBinding
// kubeadm:cluster-admins reconciliation. It evaluates three independent signals:
//
//  1. user-authz module is enabled (module.IsEnabled / global.enabledModules);
//  2. the cluster has finished its first bootstrap (global.clusterIsBootstrapped);
//  3. ClusterRole user-authz:cluster-admin is observed in the API right now.
//
// Only when all three are true the binding flips to user-authz:cluster-admin; otherwise it stays
// on cluster-admin (kubeadm-default wildcard). The supplement (extra ClusterRole bound to the same
// kubeadm:cluster-admins group) is enabled while user-authz is on (single gate).
//
// The main kubeadm:cluster-admins binding is owned entirely by this hook: it left the Helm template
// in v1.77 (Helm orphaned the live object via helm.sh/resource-policy: keep, so no binding gap ever
// opened). The hook still publishes its decision into Helm values:
//
//	controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName  string  (observability only)
//	controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled bool  (read by the template
//	                                                                          to gate the supplement)
//
// Why a hook is needed at all: ClusterRoleBinding.roleRef is immutable in Kubernetes RBAC, Helm
// SSA cannot mutate it. We Delete+Create via PatchCollector on OnBeforeHelm, before Helm runs.
// On a failed Create after Delete the next reconcile (CRB delete-event or OnBeforeHelm tick)
// hits the create-only path and self-heals.
package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	kubeadmClusterAdminsBindingName      = "kubeadm:cluster-admins"
	clusterAdminWildcardClusterRoleName  = "cluster-admin"
	userAuthzClusterAdminClusterRoleName = "user-authz:cluster-admin"

	kubeadmClusterAdminsBindingSnapshot = "kubeadm_cluster_admins_binding"
	userAuthzClusterAdminCRSnapshot     = "user_authz_cluster_admin_clusterrole"

	clusterIsBootstrappedValuePath    = "global.clusterIsBootstrapped"
	kubeadmTargetRoleNameValuePath    = "controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName"
	kubeadmSupplementEnabledValuePath = "controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled"

	// resource-policy: keep is stamped on every object the hook writes. It was the one-time guard for
	// the v1.77 converge that dropped this CRB from the Helm template: with the annotation on the live
	// object, Helm orphaned it (kept it) instead of pruning, so kubeadm:cluster-admins never lost its
	// binding and admin.conf never lost root. After that converge the object is Helm-orphaned and this
	// hook is its sole owner; the annotation is now a harmless no-op we keep for defence in depth.
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"

	// forceWildcardClusterAdmin temporarily pins kubeadm:cluster-admins to the wildcard cluster-admin
	// role, pausing the granular user-authz:cluster-admin rollout. Set to false to re-enable granular.
	forceWildcardClusterAdmin = true
)

// kubeadmClusterAdminsBindingState keeps the only moving piece of the CRB we care about.
type kubeadmClusterAdminsBindingState struct {
	RoleRefName string `json:"roleRefName"`
}

// userAuthzClusterAdminCRState is just a presence marker (snapshot length > 0 == role exists).
type userAuthzClusterAdminCRState struct {
	Name string `json:"name"`
}

func filterKubeadmClusterAdminsBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crb rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &crb); err != nil {
		return nil, fmt.Errorf("convert ClusterRoleBinding %s: %w", obj.GetName(), err)
	}
	return kubeadmClusterAdminsBindingState{
		RoleRefName: crb.RoleRef.Name,
	}, nil
}

func filterUserAuthzClusterAdminCR(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return userAuthzClusterAdminCRState{Name: obj.GetName()}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue + "/reconcile_kubeadm_cluster_admins_binding",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         kubeadmClusterAdminsBindingSnapshot,
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "ClusterRoleBinding",
			NameSelector:                 &types.NameSelector{MatchNames: []string{kubeadmClusterAdminsBindingName}},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterKubeadmClusterAdminsBinding,
		},
		{
			Name:                         userAuthzClusterAdminCRSnapshot,
			ApiVersion:                   "rbac.authorization.k8s.io/v1",
			Kind:                         "ClusterRole",
			NameSelector:                 &types.NameSelector{MatchNames: []string{userAuthzClusterAdminClusterRoleName}},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterUserAuthzClusterAdminCR,
		},
	},
}, reconcileKubeadmClusterAdminsBindingHook)

func reconcileKubeadmClusterAdminsBindingHook(_ context.Context, input *go_hook.HookInput) error {
	userAuthzEnabled := module.IsEnabled("user-authz", input)
	clusterBootstrapped := input.Values.Get(clusterIsBootstrappedValuePath).Bool()

	userAuthzCRSnaps, err := sdkobjectpatch.UnmarshalToStruct[userAuthzClusterAdminCRState](input.Snapshots, userAuthzClusterAdminCRSnapshot)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", userAuthzClusterAdminCRSnapshot, err)
	}
	userAuthzCRAvailable := len(userAuthzCRSnaps) > 0

	// TEMPORARY: the granular user-authz:cluster-admin rollout for kubeadm:cluster-admins is paused.
	// The binding is pinned back to the wildcard cluster-admin role for every cluster. The gate
	// computation below is kept intact so re-enabling is a one-liner: drop forceWildcardClusterAdmin
	// (and restore the granular test expectations).
	//
	// Safety during the rollback flip (granular -> wildcard, an immutable-roleRef Delete+Create):
	// the supplement binding stays enabled (supplementEnabled = userAuthzEnabled), so the group keeps
	// nodes/proxy (kubectl exec/logs/port-forward) and impersonate through the whole window. Wildcard
	// is a strict superset of granular, so no permission is ever lost — only the sub-second core-rbac
	// window of the Delete+Create, which the hook self-heals on the CRB delete event / next OnBeforeHelm.
	desiredRoleName := clusterAdminWildcardClusterRoleName
	if !forceWildcardClusterAdmin && userAuthzEnabled && clusterBootstrapped && userAuthzCRAvailable {
		desiredRoleName = userAuthzClusterAdminClusterRoleName
	}

	// Publish the already-made decision into values. supplementEnabled gates the supplement CRB that
	// is still template-rendered; targetRoleName is exported for observability only (the main binding
	// is hook-owned). Helm picks up values updates of OnBeforeHelm hooks before rendering.
	input.Values.Set(kubeadmTargetRoleNameValuePath, desiredRoleName)
	input.Values.Set(kubeadmSupplementEnabledValuePath, userAuthzEnabled)

	logger := input.Logger.With(
		slog.String("name", kubeadmClusterAdminsBindingName),
		slog.String("desired_role_ref", desiredRoleName),
		slog.Bool("user_authz_enabled", userAuthzEnabled),
		slog.Bool("cluster_bootstrapped", clusterBootstrapped),
		slog.Bool("user_authz_cr_available", userAuthzCRAvailable),
	)

	bindingSnaps, err := sdkobjectpatch.UnmarshalToStruct[kubeadmClusterAdminsBindingState](input.Snapshots, kubeadmClusterAdminsBindingSnapshot)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", kubeadmClusterAdminsBindingSnapshot, err)
	}

	desiredCRB := buildKubeadmClusterAdminsBinding(desiredRoleName)

	if len(bindingSnaps) == 0 {
		logger.Info("creating clusterrolebinding")
		input.PatchCollector.Create(desiredCRB)
		return nil
	}

	current := bindingSnaps[len(bindingSnaps)-1]
	if current.RoleRefName == desiredRoleName {
		// roleRef already correct: nothing to do. The hook does not touch ownership metadata — the
		// object is Helm-orphaned (or hook-created) and this hook is its sole owner. Any residual Helm
		// labels inherited from the pre-v1.77 template are left untouched; Helm no longer manages it.
		return nil
	}

	logger.Info("rebinding clusterrolebinding", slog.String("from", current.RoleRefName))
	input.PatchCollector.Delete(rbacv1.SchemeGroupVersion.String(), "ClusterRoleBinding", "", kubeadmClusterAdminsBindingName)
	input.PatchCollector.Create(desiredCRB)
	return nil
}

// buildKubeadmClusterAdminsBinding renders the desired ClusterRoleBinding state. This hook is the
// sole owner of the object (it left the Helm template in v1.77), so no Helm ownership metadata is
// set. helm.sh/resource-policy: keep is still stamped: it is the guard for the drop-from-template
// converge (if the hook (re)creates the CRB right before that Helm run, keep prevents a prune).
func buildKubeadmClusterAdminsBinding(roleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeadmClusterAdminsBindingName,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "control-plane-manager",
			},
			Annotations: map[string]string{
				helmResourcePolicyAnnotation: helmResourcePolicyKeep,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName},
		},
	}
}
