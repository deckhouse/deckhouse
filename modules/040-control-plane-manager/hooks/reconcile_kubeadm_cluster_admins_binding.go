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

// Reconciles ClusterRoleBinding kubeadm:cluster-admins when ModuleConfig user-authz changes (roleRef is immutable).
// Lives in control-plane-manager so the hook runs whenever this module is enabled (unlike user-authz-specific hooks).

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	kubeadmClusterAdminsBindingName      = "kubeadm:cluster-admins"
	clusterAdminWildcardClusterRoleName  = "cluster-admin"
	userAuthzClusterAdminClusterRoleName = "user-authz:cluster-admin"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "user_authz_module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"user-authz"},
			},
			FilterFunc: filterUserAuthzModuleConfig,
		},
	},
}, dependency.WithExternalDependencies(reconcileKubeadmClusterAdminsBindingHook))

func filterUserAuthzModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	enabled, found, err := unstructured.NestedBool(obj.Object, "spec", "enabled")
	if err != nil {
		return nil, err
	}
	if !found {
		return false, nil
	}
	return enabled, nil
}

func userAuthzEnabledFromSnapshot(input *go_hook.HookInput) bool {
	enabledSnaps, err := sdkobjectpatch.UnmarshalToStruct[bool](input.Snapshots, "user_authz_module_config")
	if err != nil || len(enabledSnaps) == 0 {
		return module.IsEnabled("user-authz", input)
	}
	return enabledSnaps[len(enabledSnaps)-1]
}

// clusterIsBootstrapped mirrors the helm template gate: until the cluster is fully bootstrapped
// (global hook cluster_is_bootstrapped sets the flag once a non-master node is Ready), keep the
// kubeadm-default wildcard binding so initial helm installs cannot fail on the immutable roleRef.
func clusterIsBootstrapped(input *go_hook.HookInput) bool {
	return input.Values.Get("global.clusterIsBootstrapped").Bool()
}

func reconcileKubeadmClusterAdminsBindingHook(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) (err error) {
	// Last-resort guard: this hook runs OnBeforeHelm and an unhandled panic here would block
	// the whole control-plane-manager helm release (and on a fresh cluster, kubectl access).
	// A panic must never escape: turn it into a logged error so addon-operator can retry safely.
	defer func() {
		if r := recover(); r != nil {
			input.Logger.Error("recovered from panic in reconcile_kubeadm_cluster_admins_binding",
				slog.Any("panic", r))
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	desiredGranular := userAuthzEnabledFromSnapshot(input) && clusterIsBootstrapped(input)
	return syncKubeadmClusterAdminsClusterRoleBinding(ctx, input.Logger, kubeCl, desiredGranular)
}

// syncKubeadmClusterAdminsClusterRoleBinding keeps ClusterRoleBinding kubeadm:cluster-admins aligned with
// the desired roleRef (roleRef is immutable, so the binding is recreated when the role changes).
// granular=true means user-authz:cluster-admin (gated by user-authz enabled and cluster bootstrapped);
// granular=false means the kubeadm-default cluster-admin wildcard.
//
// Safety properties:
//   - If granular=true but user-authz:cluster-admin does not exist yet (e.g., user-authz module is
//     enabled but its templates have not been rendered yet), we skip the rebind to avoid leaving the
//     cluster with a binding pointing at a nonexistent ClusterRole.
//   - If Delete succeeds but Create fails, we attempt a best-effort rollback to the previous binding
//     so admin.conf does not lose access while addon-operator retries the hook.
func syncKubeadmClusterAdminsClusterRoleBinding(
	ctx context.Context,
	logger go_hook.Logger,
	kubeCl kubernetes.Interface,
	granular bool,
) error {
	desiredRoleName := clusterAdminWildcardClusterRoleName
	if granular {
		desiredRoleName = userAuthzClusterAdminClusterRoleName
	}

	desiredCRB := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: kubeadmClusterAdminsBindingName,
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "ClusterRole",
			Name:     desiredRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind: rbac.GroupKind,
				Name: kubeadmClusterAdminsBindingName,
			},
		},
	}

	existing, err := kubeCl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		logger.Info("creating clusterrolebinding",
			slog.String("name", kubeadmClusterAdminsBindingName),
			slog.String("roleRef", desiredRoleName))
		if _, err := kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, desiredCRB, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
	}

	if existing.RoleRef.Name == desiredRoleName {
		return nil
	}

	if granular {
		_, err := kubeCl.RbacV1().ClusterRoles().Get(ctx, userAuthzClusterAdminClusterRoleName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			logger.Warn("desired clusterrole does not exist yet, keeping existing binding to avoid loss of access",
				slog.String("name", kubeadmClusterAdminsBindingName),
				slog.String("desired_clusterrole", userAuthzClusterAdminClusterRoleName),
				slog.String("current_roleRef", existing.RoleRef.Name))
			return nil
		}
		if err != nil {
			return fmt.Errorf("get clusterrole %s: %w", userAuthzClusterAdminClusterRoleName, err)
		}
	}

	logger.Info("rebinding clusterrolebinding",
		slog.String("name", kubeadmClusterAdminsBindingName),
		slog.String("from", existing.RoleRef.Name),
		slog.String("to", desiredRoleName))

	rollbackCRB := buildRollbackCRB(existing)

	if err := kubeCl.RbacV1().ClusterRoleBindings().Delete(ctx, kubeadmClusterAdminsBindingName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
	}

	if _, err := kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, desiredCRB, metav1.CreateOptions{}); err != nil {
		// Best-effort rollback: try to recreate the previous binding so admin.conf does not lose
		// access while the hook is retried. Failure to roll back is logged but not fatal — the next
		// hook run will reconcile from whatever state we end up in.
		if _, rbErr := kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, rollbackCRB, metav1.CreateOptions{}); rbErr != nil {
			logger.Error("rollback failed: clusterrolebinding is missing in the cluster",
				slog.String("name", kubeadmClusterAdminsBindingName),
				slog.Any("rollback_error", rbErr))
		} else {
			logger.Warn("rolled back to previous clusterrolebinding after failed rebind",
				slog.String("name", kubeadmClusterAdminsBindingName),
				slog.String("restored_roleRef", existing.RoleRef.Name))
		}
		return fmt.Errorf("create clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
	}

	return nil
}

// buildRollbackCRB returns a fresh ClusterRoleBinding object matching the immutable parts of an
// existing binding (name, roleRef, subjects). Server-managed metadata (resourceVersion, uid, etc.)
// is intentionally omitted so the object can be re-created via Create after a Delete.
func buildRollbackCRB(existing *rbac.ClusterRoleBinding) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        existing.Name,
			Labels:      existing.Labels,
			Annotations: existing.Annotations,
		},
		RoleRef:  existing.RoleRef,
		Subjects: existing.Subjects,
	}
}
