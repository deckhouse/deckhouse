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
// The hook publishes its decision into Helm values so templates/rbac-for-us.yaml renders verbatim
// and does NOT re-evaluate the gates:
//
//	controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName  string
//	controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled bool
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

	clusterIsBootstrappedValuePath           = "global.clusterIsBootstrapped"
	kubeadmTargetRoleNameValuePath           = "controlPlaneManager.internal.kubeadmClusterAdminsTargetRoleName"
	kubeadmSupplementEnabledValuePath        = "controlPlaneManager.internal.kubeadmClusterAdminsSupplementEnabled"
)

// kubeadmClusterAdminsBindingState keeps the only moving piece of the CRB — the target role.
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
	return kubeadmClusterAdminsBindingState{RoleRefName: crb.RoleRef.Name}, nil
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

	desiredRoleName := clusterAdminWildcardClusterRoleName
	if userAuthzEnabled && clusterBootstrapped && userAuthzCRAvailable {
		desiredRoleName = userAuthzClusterAdminClusterRoleName
	}

	// Single source of truth: publish the already-made decision into values so the Helm template
	// renders verbatim. Helm picks up values updates of OnBeforeHelm hooks before rendering.
	input.Values.Set(kubeadmTargetRoleNameValuePath, desiredRoleName)
	input.Values.Set(kubeadmSupplementEnabledValuePath, userAuthzEnabled)

	logger := input.Logger.With(
		slog.String("name", kubeadmClusterAdminsBindingName),
		slog.String("desiredRoleRef", desiredRoleName),
		slog.Bool("userAuthzEnabled", userAuthzEnabled),
		slog.Bool("clusterBootstrapped", clusterBootstrapped),
		slog.Bool("userAuthzCRAvailable", userAuthzCRAvailable),
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
		return nil
	}

	logger.Info("rebinding clusterrolebinding", slog.String("from", current.RoleRefName))
	input.PatchCollector.Delete(rbacv1.SchemeGroupVersion.String(), "ClusterRoleBinding", "", kubeadmClusterAdminsBindingName)
	input.PatchCollector.Create(desiredCRB)
	return nil
}

// buildKubeadmClusterAdminsBinding renders the desired ClusterRoleBinding state.
// Labels are intentionally minimal (only heritage/module): Helm SSA reconciles its full label set
// on the next pass and we deliberately do not imitate Helm-managed metadata here.
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
