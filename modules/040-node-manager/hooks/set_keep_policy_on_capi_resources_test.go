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

package hooks

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func testCRD(name string, storedVersions []string) *unstructured.Unstructured {
	stored := make([]interface{}, 0, len(storedVersions))
	for _, version := range storedVersions {
		stored = append(stored, version)
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"status": map[string]interface{}{
			"storedVersions": stored,
		},
	}}
}

func TestPickStoredVersion(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		testCRD("machinedeployments.machine.sapcloud.io", []string{"v1alpha1"}),
		testCRD("machinedeployments.cluster.x-k8s.io", []string{"v1beta1", "v1beta2"}),
		testCRD("staticmachinetemplates.infrastructure.cluster.x-k8s.io", []string{"v1alpha1"}),
	)

	version, ok, err := pickStoredVersion(dyn, "machine.sapcloud.io", "machinedeployments", mcmStoredVersions)
	if err != nil {
		t.Fatalf("pick MCM version: %v", err)
	}
	if !ok || version != "v1alpha1" {
		t.Fatalf("MCM version=%q ok=%v, want v1alpha1/true", version, ok)
	}

	version, ok, err = pickStoredVersion(dyn, "cluster.x-k8s.io", "machinedeployments", storedVersionPreference)
	if err != nil {
		t.Fatalf("pick CAPI version: %v", err)
	}
	if !ok || version != "v1beta1" {
		t.Fatalf("CAPI version=%q ok=%v, want v1beta1/true", version, ok)
	}

	version, ok, err = pickStoredVersion(dyn, "cluster.x-k8s.io", "missing", storedVersionPreference)
	if err != nil {
		t.Fatalf("missing CRD returned error: %v", err)
	}
	if ok || version != "" {
		t.Fatalf("missing CRD version=%q ok=%v, want empty/false", version, ok)
	}

	version, ok, err = pickStoredVersion(dyn, "infrastructure.cluster.x-k8s.io", "staticmachinetemplates", []string{"v1alpha1"})
	if err != nil {
		t.Fatalf("pick StaticMachineTemplate version: %v", err)
	}
	if !ok || version != "v1alpha1" {
		t.Fatalf("StaticMachineTemplate version=%q ok=%v, want v1alpha1/true", version, ok)
	}
}

func TestCapiResourcesIncludeStaticMachineTemplates(t *testing.T) {
	for _, res := range capiResources {
		if res.Group == "infrastructure.cluster.x-k8s.io" && res.Resource == "staticmachinetemplates" {
			if len(res.versionPreference) != 1 || res.versionPreference[0] != "v1alpha1" {
				t.Fatalf("StaticMachineTemplate preference=%v, want [v1alpha1]", res.versionPreference)
			}
			return
		}
	}
	t.Fatal("StaticMachineTemplate must be kept from Helm prune during migration")
}

func TestIsConversionUnavailable(t *testing.T) {
	if !isConversionUnavailable(apierrors.NewServiceUnavailable("conversion webhook unavailable")) {
		t.Fatal("service unavailable must be treated as conversion unavailable")
	}
	if !isConversionUnavailable(errors.New("object is (re)initializing, conversion webhook is not ready")) {
		t.Fatal("conversion webhook message must be treated as conversion unavailable")
	}
	if isConversionUnavailable(errors.New("forbidden")) {
		t.Fatal("unrelated errors must not be treated as conversion unavailable")
	}
}
