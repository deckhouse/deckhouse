// Copyright 2021 Flant JSC
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

package template

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	cmKind = schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
)

func fromUnstructured(unstructuredObj unstructured.Unstructured, obj interface{}) {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), obj)
	if err != nil {
		panic(err)
	}
}

func TestResourcesOrder(t *testing.T) {
	unknownAdditionalOrder := []string{
		"ClusterAuthorizationRule",
		"YandexInstanceClass",
		"NodeGroup",
	}
	expectedLen := len(unknownAdditionalOrder) + len(bootstrapKindsOrder)

	resources, err := ParseResources("testdata/resources/order.yaml", nil)
	require.NoError(t, err)

	require.Len(t, resources, expectedLen)

	t.Run("known resources should be located in begin and in order", func(t *testing.T) {
		for i, kind := range bootstrapKindsOrder {
			require.Equal(t, kind, resources[i].Object.GetKind())
		}
	})

	t.Run("unknown resources should be located after known resources in order same in yaml", func(t *testing.T) {
		for i, kind := range unknownAdditionalOrder {
			indx := i + len(bootstrapKindsOrder)
			require.Equal(t, kind, resources[indx].Object.GetKind())
		}
	})
}

func TestResourcesOrderWithSameKind(t *testing.T) {
	assertNs := func(t *testing.T, resources Resources, indx int, name string) {
		ns := v1.Namespace{}

		fromUnstructured(resources[indx].Object, &ns)
		require.Equal(t, ns.Name, name)
		require.Equal(t, resources[indx].GVK, schema.GroupVersionKind{Version: "v1", Kind: "Namespace"})
	}

	resources, err := ParseResources("testdata/resources/same_kind_order.yaml", nil)
	require.NoError(t, err)

	require.Len(t, resources, 5)

	t.Run("resources with same kind should sort on name alphanumeric order", func(t *testing.T) {
		namesInOrder := []string{
			"another",
			"r-test",
			"test-ns",
		}
		for i, name := range namesInOrder {
			assertNs(t, resources, i, name)
		}
	})
}

func TestResourcesWithTemplateData(t *testing.T) {
	const expectedValueFromCloudData = "id1"
	t.Run("parses template resources and put data in manifests", func(t *testing.T) {
		resources, err := ParseResources("testdata/resources/with_tmp.yaml", map[string]interface{}{
			"cloudDiscovery": map[string]interface{}{
				"networkId": map[string]interface{}{
					"ru-central1-a": expectedValueFromCloudData,
					"ru-central1-b": expectedValueFromCloudData + "1",
					"ru-central1-c": expectedValueFromCloudData + "2",
				},

				"anotherKey": "anotherValue",
			},
		})
		require.NoError(t, err)

		require.Len(t, resources, 1)

		cm := v1.ConfigMap{}
		fromUnstructured(resources[0].Object, &cm)

		require.Equal(t, cm.Namespace, "test-ns")
		require.Equal(t, cm.Name, "some-cm")

		require.Equal(t, cm.Data["key"], "value")
		require.Equal(t, cm.Data["fromCloudDiscovery"], expectedValueFromCloudData)
		require.Equal(t, cm.Data["sprigFuncAvailable"], "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b")

		require.Equal(t, resources[0].GVK, cmKind)
	})
}

func TestResourcesNotExistsTemplateDataReturnError(t *testing.T) {
	t.Run("returns error if value not found in data", func(t *testing.T) {
		resources, err := ParseResources("testdata/resources/with_tmp.yaml", map[string]interface{}{
			"cloudDiscovery": map[string]interface{}{
				"anotherKey": "anotherValue",
			},
		})

		require.Error(t, err)
		require.Nil(t, resources)
	})
}

func TestResourcesWithEmptyDocs(t *testing.T) {
	t.Run("returns only not empty resources", func(t *testing.T) {
		resources, err := ParseResources("testdata/resources/empties_docs.yaml", make(map[string]interface{}))

		require.NoError(t, err)
		require.Len(t, resources, 2)
	})
}
