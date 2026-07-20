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

package grants

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newAvailable(namespace, name, def string, available ...string) *unstructured.Unstructured {
	items := make([]interface{}, 0, len(available))
	for _, n := range available {
		items = append(items, map[string]interface{}{
			"name":    n,
			"default": n == def,
		})
	}

	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"default":   def,
			"available": items,
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   availableGroup,
		Version: availableVersion,
		Kind:    availableKind,
	})
	obj.SetNamespace(namespace)
	obj.SetName(name)

	return obj
}

func fakeScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   availableGroup,
		Version: availableVersion,
		Kind:    availableKind,
	}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   availableGroup,
		Version: availableVersion,
		Kind:    availableKind + "List",
	}, &unstructured.UnstructuredList{})

	return s
}

func TestResolve(t *testing.T) {
	t.Run("returns default and available names", func(t *testing.T) {
		obj := newAvailable("tenant", "storageclasses", "ssd", "ssd", "hdd")
		cli := fake.NewClientBuilder().WithScheme(fakeScheme()).WithObjects(obj).Build()

		r := NewResolver(cli)
		catalog, err := r.Resolve(context.Background(), "tenant", "storageclasses")
		require.NoError(t, err)

		assert.True(t, catalog.Found)
		assert.Equal(t, "ssd", catalog.Default)
		assert.ElementsMatch(t, []string{"ssd", "hdd"}, catalog.Available)
		assert.True(t, catalog.IsAvailable("hdd"))
		assert.False(t, catalog.IsAvailable("nvme"))
	})

	t.Run("missing object yields not found", func(t *testing.T) {
		cli := fake.NewClientBuilder().WithScheme(fakeScheme()).Build()

		r := NewResolver(cli)
		catalog, err := r.Resolve(context.Background(), "tenant", "storageclasses")
		require.NoError(t, err)

		assert.False(t, catalog.Found)
		assert.Empty(t, catalog.Default)
		assert.Empty(t, catalog.Available)
	})

	t.Run("noop resolver is always inactive", func(t *testing.T) {
		catalog, err := NoopResolver{}.Resolve(context.Background(), "tenant", "storageclasses")
		require.NoError(t, err)
		assert.False(t, catalog.Found)
	})
}
