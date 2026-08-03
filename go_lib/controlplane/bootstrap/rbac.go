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

package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	clusterAdminsBindingName          = "kubeadm:cluster-admins"
	apiserverKubeletClientBindingName = "d8:control-plane-manager:apiserver-kubelet-client"
)

// clusterAdminsBinding grants cluster-admin to the group admin.conf authenticates into.
func clusterAdminsBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: clusterAdminsBindingName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.GroupKind,
			Name:     "kubeadm:cluster-admins",
		}},
	}
}

// apiserverKubeletClientBinding lets kube-apiserver proxy kubelet requests (logs/exec/port-forward)
// during bootstrap. Module 040-control-plane-manager later adopts the same binding.
func apiserverKubeletClientBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: apiserverKubeletClientBindingName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:kubelet-api-admin",
		},
		Subjects: []rbacv1.Subject{{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.UserKind,
			Name:     "kube-apiserver-kubelet-client",
		}},
	}
}

func ensureClusterRoleBinding(ctx context.Context, client kubernetes.Interface, binding *rbacv1.ClusterRoleBinding) error {
	_, err := client.RbacV1().ClusterRoleBindings().Get(ctx, binding.Name, metav1.GetOptions{})
	if err == nil {
		logger.Info("clusterrolebinding already exists", slog.String("name", binding.Name))
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get clusterrolebinding %s: %w", binding.Name, err)
	}

	if _, err := client.RbacV1().ClusterRoleBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create clusterrolebinding %s: %w", binding.Name, err)
	}

	logger.Info("created clusterrolebinding", slog.String("name", binding.Name))

	return nil
}
