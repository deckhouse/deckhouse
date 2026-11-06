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

package hookcache

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func TestStripUnusedFields(t *testing.T) {
	managedFields := func() []any {
		return []any{map[string]any{"manager": "kubelet", "operation": "Update"}}
	}
	images := func() []any {
		return []any{map[string]any{"names": []any{"registry.example.com/app:v1"}, "sizeBytes": int64(1024)}}
	}

	t.Run("node loses managedFields and status.images and keeps the rest", func(t *testing.T) {
		node := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Node",
			"metadata":   map[string]any{"name": "worker-0", "managedFields": managedFields()},
			"status": map[string]any{
				"images":   images(),
				"nodeInfo": map[string]any{"kubeletVersion": "v1.33.4"},
			},
		}}

		// Twice: client-go may apply the transform to an object again.
		for range 2 {
			res, err := StripUnusedFields(node)
			require.NoError(t, err)
			require.Same(t, node, res)
		}

		require.Equal(t, map[string]any{
			"apiVersion": "v1",
			"kind":       "Node",
			"metadata":   map[string]any{"name": "worker-0"},
			"status":     map[string]any{"nodeInfo": map[string]any{"kubeletVersion": "v1.33.4"}},
		}, node.Object)
	})

	t.Run("other kinds keep status.images", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Node",
			"metadata":   map[string]any{"name": "app", "managedFields": managedFields()},
			"status":     map[string]any{"images": images()},
		}}

		_, err := StripUnusedFields(obj)
		require.NoError(t, err)

		require.Equal(t, map[string]any{
			"apiVersion": "example.com/v1",
			"kind":       "Node",
			"metadata":   map[string]any{"name": "app"},
			"status":     map[string]any{"images": images()},
		}, obj.Object)
	})

	t.Run("non-unstructured objects pass as is", func(t *testing.T) {
		tombstone := cache.DeletedFinalStateUnknown{Key: "default/app"}

		res, err := StripUnusedFields(tombstone)
		require.NoError(t, err)
		require.Equal(t, tombstone, res)
	})
}
