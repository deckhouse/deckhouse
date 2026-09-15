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

package derived_status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveMetal3Image(t *testing.T) {
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Metal3Image"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("Metal3ImageList"), &unstructured.UnstructuredList{})
	image := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Metal3Image",
		"metadata":   map[string]interface{}{"name": "ubuntu"},
		"spec": map[string]interface{}{
			"direct": map[string]interface{}{
				"url": "http://images.example/ubuntu.qcow2", "checksum": "abc", "checksumType": "sha256", "format": "qcow2",
			},
		},
	}}
	image.SetGroupVersionKind(gvk)
	service := &Service{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(image).Build()}
	classSpec := map[string]interface{}{
		"imageRef":     map[string]interface{}{"kind": "Metal3Image", "name": "ubuntu"},
		"hostSelector": map[string]interface{}{"matchLabels": map[string]interface{}{"pool": "workers"}},
	}

	resolved, err := service.resolveMetal3Image(t.Context(), classSpec)
	require.NoError(t, err)
	direct := resolved["image"].(map[string]interface{})
	assert.Equal(t, "http://images.example/ubuntu.qcow2", direct["url"])
	assert.Equal(t, classSpec["hostSelector"], resolved["hostSelector"])
	_, mutated := classSpec["image"]
	assert.False(t, mutated, "resolution must not mutate the cached InstanceClass object")
}
