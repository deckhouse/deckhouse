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

package template

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// bashible-apiserver has no modulesources/sensitive, so the API masks dockerCfg as "<omitted>".
// The CA must still be read.
func TestModuleSourceDecodesWithMaskedDockerCfg(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]interface{}{"name": "example"},
		"spec": map[string]interface{}{"registry": map[string]interface{}{
			"repo":      "registry.example.io/modules",
			"dockerCfg": "<omitted>",
			"ca":        "-----BEGIN CERTIFICATE-----",
		}},
	}}

	var ms ModuleSource
	if err := fromUnstructured(obj, &ms); err != nil {
		t.Fatalf("decode ModuleSource with masked dockerCfg: %v", err)
	}
	if ms.Spec.Registry.CA != "-----BEGIN CERTIFICATE-----" {
		t.Fatalf("CA = %q, want the certificate", ms.Spec.Registry.CA)
	}
	if got := ms.getRegistry(); got != "registry.example.io" {
		t.Fatalf("registry = %q, want registry.example.io", got)
	}
}
