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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMergeCacheByObject_UnionsNamespacesOfOneKind(t *testing.T) {
	t.Parallel()

	selector := labels.SelectorFromSet(labels.Set{"app": "x"})
	got, err := MergeCacheByObject(
		map[client.Object]cache.ByObject{
			&corev1.ConfigMap{}:     {Namespaces: map[string]cache.Config{"a": {LabelSelector: selector}}},
			Object(OAuth2ClientGVK): {Namespaces: map[string]cache.Config{"a": {}}},
		},
		map[client.Object]cache.ByObject{
			&corev1.ConfigMap{}:     {Namespaces: map[string]cache.Config{"b": {}, "a": {LabelSelector: labels.SelectorFromSet(labels.Set{"app": "x"})}}},
			Object(DexClientGVK):    {},
			Object(OAuth2ClientGVK): {Namespaces: map[string]cache.Config{"a": {}}},
		},
	)
	if err != nil {
		t.Fatalf("MergeCacheByObject: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d kinds, want 3: %v", len(got), got)
	}
	for obj, cfg := range got {
		if _, ok := obj.(*corev1.ConfigMap); !ok {
			continue
		}
		if len(cfg.Namespaces) != 2 || cfg.Namespaces["a"].LabelSelector.String() != selector.String() {
			t.Errorf("ConfigMap namespaces = %v", cfg.Namespaces)
		}
	}
}

func TestMergeCacheByObject_Conflicts(t *testing.T) {
	t.Parallel()

	byName := func(name string) cache.Config {
		return cache.Config{FieldSelector: fields.OneTermEqualSelector("metadata.name", name)}
	}
	for name, parts := range map[string][]map[client.Object]cache.ByObject{
		"namespace scoped differently": {
			{&corev1.Secret{}: {Namespaces: map[string]cache.Config{"a": byName("x")}}},
			{&corev1.Secret{}: {Namespaces: map[string]cache.Config{"a": byName("y")}}},
		},
		"top-level selectors differ": {
			{&corev1.Secret{}: {Label: labels.SelectorFromSet(labels.Set{"a": "1"})}},
			{&corev1.Secret{}: {}},
		},
		"transform": {
			{Object(OAuth2ClientGVK): {Transform: StripDexSecrets}},
			{Object(OAuth2ClientGVK): {}},
		},
		"namespace next to all namespaces": {
			{&corev1.Secret{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}}},
			{&corev1.Secret{}: {Namespaces: map[string]cache.Config{"a": byName("x")}}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := MergeCacheByObject(parts...); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
