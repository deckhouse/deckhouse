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

package capi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/register"
)

var capiGV = schema.GroupVersion{Group: "cluster.x-k8s.io", Version: "v1beta2"}

func clusterReconciler(t *testing.T, objs ...runtime.Object) (*ClusterReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, deckhousev1.AddToScheme(scheme))
	scheme.AddKnownTypeWithName(capiGV.WithKind("Cluster"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(capiGV.WithKind("MachineHealthCheck"), &unstructured.Unstructured{})

	objs = append(objs, clusterConfigSecret(
		"podSubnetCIDR: 10.111.0.0/16\nserviceSubnetCIDR: 10.222.0.0/16\nclusterDomain: cluster.local\n"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

	return &ClusterReconciler{BaseWithReader: BaseWithReader{
		Base:      register.Base{Client: c},
		APIReader: c,
	}}, c
}

func getCluster(t *testing.T, c client.Client, name string) (*unstructured.Unstructured, error) {
	t.Helper()
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(capiGV.WithKind("Cluster"))
	err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: capiNamespace}, cluster)
	return cluster, err
}

func staticNodeGroup(name string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:        deckhousev1.NodeTypeStatic,
			StaticInstances: &deckhousev1.StaticInstancesSpec{Count: ptr(int32(1))},
		},
	}
}

// A cluster with no cloud provider produces no registration key, so the NodeGroup is the only
// trigger the static Cluster has.
func TestReconcile_StaticClusterIsEnsuredWithoutAnyRegistration(t *testing.T) {
	r, c := clusterReconciler(t, staticNodeGroup("worker"))

	// The key a NodeGroup event produces.
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "worker"},
	})
	require.NoError(t, err)

	cluster, err := getCluster(t, c, "static")
	require.NoError(t, err)
	assert.Equal(t, "caps-controller-manager", cluster.GetLabels()["app"])
}

// One key, one Cluster: a registration key must not also ensure the static one.
func TestReconcile_RegistrationKeyEnsuresTheCloudClusterOnly(t *testing.T) {
	registration := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cloudprovider.SecretNamespace,
			Name:      cloudprovider.SecretNamePrefix + "-yandex",
			Labels:    map[string]string{cloudprovider.SecretLabel: ""},
		},
		Data: map[string][]byte{
			"type":            []byte("yandex"),
			"capiClusterName": []byte("yandex"),
			"capiClusterKind": []byte("YandexCluster"),
		},
	}
	r, c := clusterReconciler(t, registration, staticNodeGroup("worker"))

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: cloudprovider.SecretNamespace,
			Name:      registration.Name,
		},
	})
	require.NoError(t, err)

	cluster, err := getCluster(t, c, "yandex")
	require.NoError(t, err)
	assert.Equal(t, "capi-controller-manager", cluster.GetLabels()["app"])

	_, err = getCluster(t, c, "static")
	assert.True(t, apierrors.IsNotFound(err), "the static Cluster belongs to a NodeGroup key")
}
