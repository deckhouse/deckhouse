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

func reconcileKubeadmClusterAdminsBindingHook(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	return syncKubeadmClusterAdminsClusterRoleBinding(ctx, input.Logger, kubeCl, userAuthzEnabledFromSnapshot(input))
}

// syncKubeadmClusterAdminsClusterRoleBinding keeps ClusterRoleBinding kubeadm:cluster-admins aligned with
// user-authz enablement (roleRef is immutable, so the binding is recreated when the desired role changes).
func syncKubeadmClusterAdminsClusterRoleBinding(
	ctx context.Context,
	logger go_hook.Logger,
	kubeCl kubernetes.Interface,
	userAuthzModuleEnabled bool,
) error {
	desiredRoleName := clusterAdminWildcardClusterRoleName
	if userAuthzModuleEnabled {
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
		_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, desiredCRB, metav1.CreateOptions{})
		if err != nil {
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

	logger.Info("rebinding clusterrolebinding",
		slog.String("name", kubeadmClusterAdminsBindingName),
		slog.String("from", existing.RoleRef.Name),
		slog.String("to", desiredRoleName))

	err = kubeCl.RbacV1().ClusterRoleBindings().Delete(ctx, kubeadmClusterAdminsBindingName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("delete clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
	}

	_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, desiredCRB, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create clusterrolebinding %s: %w", kubeadmClusterAdminsBindingName, err)
	}

	return nil
}
