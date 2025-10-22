/*
Copyright 2023 Flant JSC

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

package ensure_crds

import (
	"context"
	"sort"
	"testing"

	"github.com/flant/kube-client/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimachineryv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func TestEnsureCRDs(t *testing.T) {
	cluster := fake.NewFakeCluster(fake.ClusterVersionV125)
	dependency.TestDC.K8sClient = cluster.Client

	merr := EnsureCRDs("./test_data/**", dependency.TestDC)
	assert.Errorf(t, merr, "invalid CRD document apiversion/kind: 'v1/Pod'")

	list, err := cluster.Client.Dynamic().Resource(crdGVR).List(context.TODO(), apimachineryv1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 5)

	expected := []string{
		"deschedulers.deckhouse.io",
		"modulereleases.deckhouse.io",
		"modules.deckhouse.io",
		"modulesources.deckhouse.io",
		"prometheuses.monitoring.coreos.com",
	}

	result := make([]string, 0, len(expected))
	for _, item := range list.Items {
		require.Equal(t, true, item.GetLabels()["heritage"] == "deckhouse")
		result = append(result, item.GetName())
	}
	sort.Strings(result)
	assert.Equal(t, expected, result)
}

func TestDeleteCRDs(t *testing.T) {
	cluster := fake.NewFakeCluster(fake.ClusterVersionV125)
	cluster.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleSource", true)

	client := cluster.Client.Dynamic()
	inst, err := NewCRDsInstaller(cluster.Client, "./test_data/single.crd")
	require.NoError(t, err)

	merr := inst.Run(context.TODO())
	assert.Equal(t, merr, nil)

	gvr := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "modulesources",
	}

	msObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "ModuleSource",
			"metadata": map[string]interface{}{
				"name": "some-object",
			},
		},
	}

	_, err = client.Resource(gvr).Create(context.TODO(), msObject, apimachineryv1.CreateOptions{})
	require.NoError(t, err)

	// one cr is in the cluster
	list, err := client.Resource(crdGVR).List(context.TODO(), apimachineryv1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	// no crds should be deleted
	deleted, err := inst.DeleteCRDs(context.TODO(), []string{"modulesources.deckhouse.io"})
	require.NoError(t, err)
	require.Len(t, deleted, 0)

	list, err = client.Resource(crdGVR).List(context.TODO(), apimachineryv1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	expected := []string{
		"modulesources.deckhouse.io",
	}

	result := make([]string, 0, len(expected))
	for _, item := range list.Items {
		result = append(result, item.GetName())
	}
	sort.Strings(result)
	assert.Equal(t, expected, result)

	// no cr in the cluster
	err = client.Resource(gvr).Delete(context.TODO(), msObject.GetName(), apimachineryv1.DeleteOptions{})
	require.NoError(t, err)

	// one crd should be deleted
	deleted, err = inst.DeleteCRDs(context.TODO(), []string{"modulesources.deckhouse.io"})
	require.NoError(t, err)
	require.Len(t, deleted, 1)

	expected = []string{
		"modulesources.deckhouse.io",
	}
	assert.Equal(t, expected, deleted)

	// no crds left
	list, err = client.Resource(crdGVR).List(context.TODO(), apimachineryv1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 0)
}
