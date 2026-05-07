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

// reconcile_kubeadm_cluster_admins_binding rebinds ClusterRoleBinding kubeadm:cluster-admins
// between cluster-admin and user-authz:cluster-admin when the user-authz module is toggled.
// roleRef is immutable, so Helm SSA can not do it — we Delete+Create via PatchCollector
// on OnBeforeHelm, before Helm runs. The desired roleRef is derived from the same signal
// templates/rbac-for-us.yaml uses (module.IsEnabled("user-authz")) to keep them in sync.
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
)

// kubeadmClusterAdminsBindingState is what the snapshot keeps for this hook —
// only the moving piece (RoleRef.Name) since everything else (subjects, name)
// is invariant for kubeadm:cluster-admins.
type kubeadmClusterAdminsBindingState struct {
	RoleRefName string `json:"roleRefName"`
}

func filterKubeadmClusterAdminsBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var crb rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &crb); err != nil {
		return nil, fmt.Errorf("convert ClusterRoleBinding %s: %w", obj.GetName(), err)
	}
	return kubeadmClusterAdminsBindingState{RoleRefName: crb.RoleRef.Name}, nil
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
	},
}, reconcileKubeadmClusterAdminsBindingHook)

func reconcileKubeadmClusterAdminsBindingHook(_ context.Context, input *go_hook.HookInput) error {
	desiredRoleName := clusterAdminWildcardClusterRoleName
	if module.IsEnabled("user-authz", input) {
		desiredRoleName = userAuthzClusterAdminClusterRoleName
	}

	logger := input.Logger.With(
		slog.String("name", kubeadmClusterAdminsBindingName),
		slog.String("desiredRoleRef", desiredRoleName),
	)

	states, err := sdkobjectpatch.UnmarshalToStruct[kubeadmClusterAdminsBindingState](input.Snapshots, kubeadmClusterAdminsBindingSnapshot)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", kubeadmClusterAdminsBindingSnapshot, err)
	}

	desiredCRB := buildKubeadmClusterAdminsBinding(desiredRoleName)

	if len(states) == 0 {
		logger.Info("creating clusterrolebinding")
		input.PatchCollector.Create(desiredCRB)
		return nil
	}

	current := states[len(states)-1]
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
