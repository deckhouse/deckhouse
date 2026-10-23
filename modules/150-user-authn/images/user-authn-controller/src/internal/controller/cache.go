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

package controller

import (
	"fmt"
	"maps"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MergeCacheByObject combines the informer settings of several reconcilers. The map is keyed by
// object pointers, so two reconcilers watching the same kind would each add a key of their own
// and one of them would silently lose its scope. Settings of one kind are merged instead: the
// namespaces of both are kept, and a namespace or a top-level selector set differently by two
// of them is an error, since an informer has one selector. So is a namespace next to
// cache.AllNamespaces: its own settings would replace the cluster-wide ones there.
func MergeCacheByObject(parts ...map[client.Object]cache.ByObject) (map[client.Object]cache.ByObject, error) {
	type entry struct {
		obj client.Object
		cfg cache.ByObject
	}
	byKind := map[string]*entry{}
	var order []string

	for _, part := range parts {
		for obj, cfg := range part {
			key := cacheKindKey(obj)
			have, ok := byKind[key]
			if !ok {
				cfg.Namespaces = maps.Clone(cfg.Namespaces)
				byKind[key] = &entry{obj: obj, cfg: cfg}
				order = append(order, key)
				continue
			}
			merged, err := mergeByObject(have.cfg, cfg)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			have.cfg = merged
		}
	}

	for _, key := range order {
		if err := checkAllNamespaces(byKind[key].cfg.Namespaces); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
	}

	out := make(map[client.Object]cache.ByObject, len(byKind))
	for _, key := range order {
		out[byKind[key].obj] = byKind[key].cfg
	}
	return out, nil
}

func mergeByObject(a, b cache.ByObject) (cache.ByObject, error) {
	if selectorString(a.Label) != selectorString(b.Label) || fieldString(a.Field) != fieldString(b.Field) {
		return a, fmt.Errorf("different top-level selectors")
	}
	if a.Transform != nil || b.Transform != nil {
		return a, fmt.Errorf("a transform cannot be merged")
	}
	if a.Namespaces == nil {
		a.Namespaces = map[string]cache.Config{}
	}
	for ns, cfg := range b.Namespaces {
		have, ok := a.Namespaces[ns]
		if !ok {
			a.Namespaces[ns] = cfg
			continue
		}
		if selectorString(have.LabelSelector) != selectorString(cfg.LabelSelector) ||
			fieldString(have.FieldSelector) != fieldString(cfg.FieldSelector) ||
			have.Transform != nil || cfg.Transform != nil {
			return a, fmt.Errorf("namespace %q is scoped differently", ns)
		}
	}
	return a, nil
}

func checkAllNamespaces(namespaces map[string]cache.Config) error {
	if _, ok := namespaces[cache.AllNamespaces]; !ok {
		return nil
	}
	for ns := range namespaces {
		if ns != cache.AllNamespaces {
			return fmt.Errorf("namespace %q overrides the cluster-wide settings", ns)
		}
	}
	return nil
}

func cacheKindKey(obj client.Object) string {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.GroupVersionKind().String()
	}
	return reflect.TypeOf(obj).String()
}

func selectorString(s labels.Selector) string {
	if s == nil {
		return ""
	}
	return s.String()
}

func fieldString(s fields.Selector) string {
	if s == nil {
		return ""
	}
	return s.String()
}
