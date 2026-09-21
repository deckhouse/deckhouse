// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

// impersonationDenied is what the kubectl proxy relays while the binding that lets
// kubernetes-admin impersonate is unavailable — an apiserver restarting, or
// control-plane-manager re-creating the kubeadm:cluster-admins ClusterRoleBinding.
const impersonationDenied = `users "dhctl" is forbidden: User "kubernetes-admin" cannot impersonate resource "users" in API group "" at the cluster scope`

func statusErr(code int32, reason metav1.StatusReason, message string) error {
	return &apierrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    code,
		Reason:  reason,
		Message: message,
	}}
}

// newFakeKubeClientFailingNodeList returns a client whose Nodes().List() always fails with
// listErr.
func newFakeKubeClientFailingNodeList(t *testing.T, listErr error) *client.KubernetesClient {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClient()

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	clientset.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, listErr
	})

	return kubeCl
}

func TestCheckControlPlaneNodesReadyAuthErrors(t *testing.T) {
	tests := []struct {
		name    string
		listErr error
	}{
		{
			name:    "impersonation denied",
			listErr: statusErr(403, metav1.StatusReasonForbidden, impersonationDenied),
		},
		{
			name:    "unauthorized",
			listErr: statusErr(401, metav1.StatusReasonUnauthorized, "Unauthorized"),
		},
	}

	for _, tt := range tests {
		// Over the SSH tunnel dhctl is system:masters, so the denial is the apiserver not being
		// ready to answer: keep polling.
		t.Run(tt.name+" is retriable over kube-proxy", func(t *testing.T) {
			kubeCl := newFakeKubeClientFailingNodeList(t, tt.listErr)
			ctx := kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeKubeProxy)

			_, err := checkControlPlaneNodesReady(ctx, kubeCl, nil)

			require.ErrorIs(t, err, ErrControlPlaneReadinessCheckTransient)
		})

		// With --kubeconfig the same answer is a verdict about those credentials, so the loop
		// must stop instead of spending its 500 attempts.
		t.Run(tt.name+" stops the loop with own credentials", func(t *testing.T) {
			kubeCl := newFakeKubeClientFailingNodeList(t, tt.listErr)
			ctx := kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeOwnCredentials)

			_, err := checkControlPlaneNodesReady(ctx, kubeCl, nil)

			require.Error(t, err)
			require.NotErrorIs(t, err, ErrControlPlaneReadinessCheckTransient)
		})
	}
}

func TestCheckControlPlaneNodesReadyTransportError(t *testing.T) {
	// A transport failure is transient regardless of how we authenticate.
	for _, mode := range []kubeerrors.AuthMode{kubeerrors.AuthModeKubeProxy, kubeerrors.AuthModeOwnCredentials} {
		t.Run(mode.String(), func(t *testing.T) {
			kubeCl := newFakeKubeClientFailingNodeList(t,
				errors.New("dial tcp 127.0.0.1:6445: connect: connection refused"))
			ctx := kubeerrors.WithAuthMode(t.Context(), mode)

			_, err := checkControlPlaneNodesReady(ctx, kubeCl, nil)

			require.ErrorIs(t, err, ErrControlPlaneReadinessCheckTransient)
		})
	}
}

// IsReadyAll must not abort on the impersonation denial that started this: it keeps polling and
// succeeds as soon as the apiserver serves the list again.
// The master on its way out is not required to answer for itself: it is the one being
// removed, and its ControlPlaneNode is what the removal is often about.
func TestCheckControlPlaneNodesReadyExcludesTheLeavingMaster(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group: "control-plane.deckhouse.io", Version: "v1alpha1", Resource: "controlplanenodes",
	}
	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		gvr: "ControlPlaneNodeList",
	})

	for _, nodeName := range []string{"master-0", "master-1", "master-2"} {
		_, err := kubeCl.CoreV1().Nodes().Create(t.Context(), &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{"node.deckhouse.io/group": "master"},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		conditions := make([]any, 0, len(requiredControlPlaneNodeConditions))
		for _, conditionType := range requiredControlPlaneNodeConditions {
			status := string(metav1.ConditionTrue)
			if nodeName == "master-2" && conditionType == "APIServerReady" {
				status = string(metav1.ConditionFalse)
			}
			conditions = append(conditions, map[string]any{"type": conditionType, "status": status})
		}

		_, err = kubeCl.Dynamic().Resource(gvr).Namespace("kube-system").Create(t.Context(), &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "control-plane.deckhouse.io/v1alpha1",
				"kind":       "ControlPlaneNode",
				"metadata":   map[string]any{"name": nodeName, "namespace": "kube-system"},
				"status":     map[string]any{"conditions": conditions},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	_, err := checkControlPlaneNodesReady(t.Context(), kubeCl, nil)
	require.ErrorIs(t, err, ErrControlPlaneIsNotReady)

	msg, err := checkControlPlaneNodesReady(t.Context(), kubeCl, []string{"master-2"})
	require.NoError(t, err)
	require.Contains(t, msg, "Ready 2 of 2")
	require.NotContains(t, msg, "master-2")
}

func TestManagerReadinessCheckerIsReadyAllExceptSkipsExcludedUnreadyNode(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group: "control-plane.deckhouse.io", Version: "v1alpha1", Resource: "controlplanenodes",
	}
	kubeCl := client.NewFakeKubernetesClientWithListGVR(map[schema.GroupVersionResource]string{
		gvr: "ControlPlaneNodeList",
	})

	for _, nodeName := range []string{"master-0", "master-1"} {
		_, err := kubeCl.CoreV1().Nodes().Create(t.Context(), &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nodeName,
				Labels: map[string]string{"node.deckhouse.io/group": "master"},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		conditions := make([]any, 0, len(requiredControlPlaneNodeConditions))
		for _, conditionType := range requiredControlPlaneNodeConditions {
			status := string(metav1.ConditionTrue)
			if nodeName == "master-1" && conditionType == "APIServerReady" {
				status = string(metav1.ConditionFalse)
			}
			conditions = append(conditions, map[string]any{"type": conditionType, "status": status})
		}

		_, err = kubeCl.Dynamic().Resource(gvr).Namespace("kube-system").Create(t.Context(), &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "control-plane.deckhouse.io/v1alpha1",
				"kind":       "ControlPlaneNode",
				"metadata":   map[string]any{"name": nodeName, "namespace": "kube-system"},
				"status":     map[string]any{"conditions": conditions},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	checker := NewManagerReadinessChecker(fakeKubeClientGetter{kubeCl: kubeCl})
	require.NoError(t, checker.IsReadyAllExcept(ctx, "master-1"))
}

func TestIsReadyAllRidesOutImpersonationDenial(t *testing.T) {
	const failedAttempts = 3

	kubeCl := client.NewFakeKubernetesClient()
	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	attempts := 0
	clientset.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts <= failedAttempts {
			return true, nil, statusErr(403, metav1.StatusReasonForbidden, impersonationDenied)
		}
		// No master nodes: readiness is trivially satisfied, which ends the loop.
		return false, nil, nil
	})

	checker := NewManagerReadinessChecker(fakeKubeClientGetter{kubeCl: kubeCl})

	ctx := kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeKubeProxy)

	require.NoError(t, checker.IsReadyAll(ctx))
	require.Greater(t, attempts, failedAttempts)
}

// The same loop with --kubeconfig credentials gives up on the first denial.
func TestIsReadyAllStopsOnDenialWithOwnCredentials(t *testing.T) {
	kubeCl := client.NewFakeKubernetesClient()

	clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
	require.True(t, ok, "fake kube client is not backed by a fake clientset")

	attempts := 0
	clientset.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, statusErr(403, metav1.StatusReasonForbidden,
			`nodes is forbidden: User "limited" cannot list resource "nodes" in API group "" at the cluster scope`)
	})

	checker := NewManagerReadinessChecker(fakeKubeClientGetter{kubeCl: kubeCl})

	ctx := kubeerrors.WithAuthMode(t.Context(), kubeerrors.AuthModeOwnCredentials)

	require.Error(t, checker.IsReadyAll(ctx))
	require.Equal(t, 1, attempts, "a permission verdict must not be retried")
}

type fakeKubeClientGetter struct {
	kubeCl *client.KubernetesClient
}

func (g fakeKubeClientGetter) KubeClientCtx(context.Context) (*client.KubernetesClient, error) {
	return g.kubeCl, nil
}
