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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
)

const testNodeName = "test-master-0"

func testOptions(pkiDir string) EnsureClusterObjectsOptions {
	return EnsureClusterObjectsOptions{
		PKIDir:                   pkiDir,
		NodeRegistrationTimeout:  time.Second,
		NodeRegistrationInterval: 5 * time.Millisecond,
	}
}

func registeredNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNodeName,
			Labels: map[string]string{"kubernetes.io/hostname": testNodeName},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: "custom", Value: "value", Effect: corev1.TaintEffectNoExecute}},
		},
	}
}

func getClusterRoleBinding(t *testing.T, client *fake.Clientset, name string) *rbacv1.ClusterRoleBinding {
	t.Helper()

	binding, err := client.RbacV1().ClusterRoleBindings().Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)

	return binding
}

func getNode(t *testing.T, client *fake.Clientset) *corev1.Node {
	t.Helper()

	node, err := client.CoreV1().Nodes().Get(context.Background(), testNodeName, metav1.GetOptions{})
	require.NoError(t, err)

	return node
}

func TestEnsureClusterObjects(t *testing.T) {
	client := fake.NewClientset(registeredNode())
	pkiDir := writePKIDir(t, nil)

	require.NoError(t, EnsureClusterObjects(context.Background(), client, testNodeName, testOptions(pkiDir)))

	clusterAdmins := getClusterRoleBinding(t, client, clusterAdminsBindingName)
	require.Equal(t, "cluster-admin", clusterAdmins.RoleRef.Name)
	require.Equal(t, "ClusterRole", clusterAdmins.RoleRef.Kind)
	require.Equal(t, []rbacv1.Subject{{
		APIGroup: rbacv1.GroupName,
		Kind:     rbacv1.GroupKind,
		Name:     "kubeadm:cluster-admins",
	}}, clusterAdmins.Subjects)

	kubeletClient := getClusterRoleBinding(t, client, apiserverKubeletClientBindingName)
	require.Equal(t, "system:kubelet-api-admin", kubeletClient.RoleRef.Name)
	require.Equal(t, []rbacv1.Subject{{
		APIGroup: rbacv1.GroupName,
		Kind:     rbacv1.UserKind,
		Name:     "kube-apiserver-kubelet-client",
	}}, kubeletClient.Subjects)

	node := getNode(t, client)
	require.Contains(t, node.Labels, constants.ControlPlaneLabelKey)
	require.Equal(t, testNodeName, node.Labels["kubernetes.io/hostname"])
	require.Contains(t, node.Spec.Taints, corev1.Taint{
		Key:    constants.ControlPlaneTaintKey,
		Effect: corev1.TaintEffectNoSchedule,
	})
	require.Contains(t, node.Spec.Taints, corev1.Taint{Key: "custom", Value: "value", Effect: corev1.TaintEffectNoExecute})

	require.Len(t, getPKISecret(t, client).Data, len(requiredPKIFiles))
}

func TestEnsureClusterObjectsIsIdempotent(t *testing.T) {
	client := fake.NewClientset(registeredNode())
	pkiDir := writePKIDir(t, nil)
	ctx := context.Background()

	require.NoError(t, EnsureClusterObjects(ctx, client, testNodeName, testOptions(pkiDir)))
	secretBefore := getPKISecret(t, client)
	nodeBefore := getNode(t, client)

	client.ClearActions()
	require.NoError(t, EnsureClusterObjects(ctx, client, testNodeName, testOptions(pkiDir)))

	// Node marking always patches, everything else must stay untouched on a repeated run.
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" || action.GetResource().Resource == "nodes" {
			continue
		}
		t.Errorf("unexpected %s on %s during a repeated run", action.GetVerb(), action.GetResource().Resource)
	}

	require.Equal(t, secretBefore.Data, getPKISecret(t, client).Data)
	require.Equal(t, nodeBefore.Labels, getNode(t, client).Labels)
	require.ElementsMatch(t, nodeBefore.Spec.Taints, getNode(t, client).Spec.Taints)
}

func TestEnsureClusterObjectsKeepsExistingClusterRoleBinding(t *testing.T) {
	adopted := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   apiserverKubeletClientBindingName,
			Labels: map[string]string{"heritage": "deckhouse"},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "system:kubelet-api-admin"},
	}
	client := fake.NewClientset(registeredNode(), adopted)
	pkiDir := writePKIDir(t, nil)

	require.NoError(t, EnsureClusterObjects(context.Background(), client, testNodeName, testOptions(pkiDir)))

	binding := getClusterRoleBinding(t, client, apiserverKubeletClientBindingName)
	require.Equal(t, "deckhouse", binding.Labels["heritage"])
	require.Empty(t, binding.Subjects)
}

func TestEnsureClusterObjectsEmptyNodeName(t *testing.T) {
	err := EnsureClusterObjects(context.Background(), fake.NewClientset(), "", EnsureClusterObjectsOptions{})
	require.ErrorContains(t, err, "empty node name")
}

func TestEnsureClusterObjectsNodeNeverRegisters(t *testing.T) {
	client := fake.NewClientset()
	pkiDir := writePKIDir(t, nil)

	opts := testOptions(pkiDir)
	opts.NodeRegistrationTimeout = 50 * time.Millisecond

	err := EnsureClusterObjects(context.Background(), client, testNodeName, opts)
	require.ErrorContains(t, err, "wait for node "+testNodeName+" registration")
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// The first binding is created before the wait, the PKI secret only after it.
	getClusterRoleBinding(t, client, clusterAdminsBindingName)
	_, secretErr := client.CoreV1().Secrets(pkiSecretNamespace).Get(context.Background(), pkiSecretName, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(secretErr))
}

func TestWaitForNodeRegistrationRetriesUntilNodeAppears(t *testing.T) {
	client := fake.NewClientset(registeredNode())

	attempts := 0
	client.PrependReactor("get", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts < 3 {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, testNodeName)
		}

		return false, nil, nil
	})

	err := waitForNodeRegistration(context.Background(), client, testNodeName, time.Second, 5*time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}
