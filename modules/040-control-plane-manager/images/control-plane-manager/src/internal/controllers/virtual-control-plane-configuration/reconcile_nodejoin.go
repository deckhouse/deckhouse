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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/bootstraptoken"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcileNodeJoinSecret ensures a valid bootstrap-token and node-bootstrapper RBAC exist in the tenant cluster.
// Returns the "id.secret" token for join.sh rendering.
func (r *reconciler) reconcileNodeJoinSecret(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (string, reconcile.Result, error) {
	ts, err := r.tenantClientset(ctx, vcp)
	if err != nil {
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	scopeLabels := map[string]string{
		constants.HeritageLabelKey: constants.HeritageLabelValue,
		"module":                   constants.ControlPlaneManagerName,
		constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
	}
	selector := fmt.Sprintf("%s=%s", constants.VirtualControlPlaneScopeLabelKey, vcp.Name)

	token, err := bootstraptoken.EnsureValid(
		ctx, ts, selector,
		[]string{constants.VirtualBootstrapTokenGroup},
		constants.VirtualBootstrapTokenTTL,
		constants.VirtualBootstrapTokenRegenBelow,
		scopeLabels,
	)
	if err != nil {
		return "", reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	if err := ensureBootstrapRBAC(ctx, ts); err != nil {
		return "", reconcile.Result{}, fmt.Errorf("ensure bootstrap RBAC: %w", err)
	}

	return token, reconcile.Result{}, nil
}

func ensureBootstrapRBAC(ctx context.Context, ts kubernetes.Interface) error {
	bindings := []*rbacv1.ClusterRoleBinding{
		clusterRoleBinding("d8-vcp-node-bootstrapper", "system:node-bootstrapper",
			rbacv1.Subject{Kind: "Group", Name: constants.VirtualBootstrapTokenGroup, APIGroup: rbacv1.GroupName}),
		clusterRoleBinding("d8-vcp-node-autoapprove-bootstrap",
			"system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			rbacv1.Subject{Kind: "Group", Name: constants.VirtualBootstrapTokenGroup, APIGroup: rbacv1.GroupName}),
		clusterRoleBinding("d8-vcp-node-autoapprove-certificate-rotation",
			"system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			rbacv1.Subject{Kind: "Group", Name: "system:nodes", APIGroup: rbacv1.GroupName}),
	}
	for _, b := range bindings {
		_, err := ts.RbacV1().ClusterRoleBindings().Create(ctx, b, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create CRB %s: %w", b.Name, err)
		}
	}
	return nil
}

func clusterRoleBinding(name, clusterRole string, subject rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{constants.HeritageLabelKey: constants.HeritageLabelValue},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: []rbacv1.Subject{subject},
	}
}
