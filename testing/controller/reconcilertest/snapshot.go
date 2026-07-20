// Copyright 2025 Flant JSC
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

package reconcilertest

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ObjectNormalizer mutates a listed object in place before it is marshalled,
// used to stabilise non-deterministic fields (timestamps, generated messages, ...).
type ObjectNormalizer func(client.Object)

// BytesNormalizer post-processes the serialized snapshot, used for normalizations
// that are easier to express as text substitutions (e.g. regex on timestamps).
type BytesNormalizer func([]byte) []byte

// SnapshotSpec describes how to dump cluster state into a stable YAML snapshot.
type SnapshotSpec struct {
	// Kinds lists, in order, the resource kinds to include in the snapshot.
	Kinds []schema.GroupVersionKind
	// ObjectNormalizers are applied to every listed object before marshalling.
	ObjectNormalizers []ObjectNormalizer
	// BytesNormalizers are applied to the full serialized output.
	BytesNormalizers []BytesNormalizer
}

// Snapshot lists every requested kind from the cluster, normalizes the objects,
// marshals them to YAML and joins them with `---` separators. The output is
// byte-compatible with the hand-written `fetchResults` helpers it replaces:
// objects are listed in the same order, get their GVK set, and are marshalled
// with sigs.k8s.io/yaml.
func Snapshot(ctx context.Context, cl client.Client, scheme *runtime.Scheme, spec SnapshotSpec) ([]byte, error) {
	result := bytes.NewBuffer(nil)

	for _, gvk := range spec.Kinds {
		listGVK := gvk
		listGVK.Kind = gvk.Kind + "List"

		listObj, err := scheme.New(listGVK)
		if err != nil {
			return nil, fmt.Errorf("new list for %s: %w", listGVK, err)
		}

		list, ok := listObj.(client.ObjectList)
		if !ok {
			return nil, fmt.Errorf("%T is not a client.ObjectList", listObj)
		}

		if err := cl.List(ctx, list); err != nil {
			return nil, fmt.Errorf("list %s: %w", listGVK, err)
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			return nil, fmt.Errorf("extract %s items: %w", listGVK, err)
		}

		for _, item := range items {
			obj, ok := item.(client.Object)
			if !ok {
				return nil, fmt.Errorf("listed item %T is not a client.Object", item)
			}

			obj.GetObjectKind().SetGroupVersionKind(gvk)
			for _, normalize := range spec.ObjectNormalizers {
				normalize(obj)
			}

			marshalled, err := yaml.Marshal(obj)
			if err != nil {
				return nil, fmt.Errorf("marshal %s: %w", gvk, err)
			}

			result.WriteString("---\n")
			result.Write(marshalled)
		}
	}

	out := result.Bytes()
	for _, normalize := range spec.BytesNormalizers {
		out = normalize(out)
	}

	return out, nil
}
