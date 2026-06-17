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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractPublishAPISettingsFromMC_NormalizesLegacyEnable(t *testing.T) {
	t.Parallel()

	moduleConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"settings": map[string]interface{}{
					"publishAPI": map[string]interface{}{
						"enable":                      true,
						"ingressClass":                "nginx",
						"addKubeconfigGeneratorEntry": true,
					},
				},
			},
		},
	}

	settings, exists, err := extractPublishAPISettingsFromMC(moduleConfig)
	if err != nil {
		t.Fatalf("extractPublishAPISettingsFromMC returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected publishAPI settings to exist")
	}
	if legacyValue, hasLegacy := settings["enable"]; hasLegacy {
		t.Fatalf("expected legacy enable key to be removed, got %#v", legacyValue)
	}
	if enabledValue, hasEnabled := settings["enabled"]; !hasEnabled || enabledValue != true {
		t.Fatalf("expected enabled=true after normalization, got %#v", settings["enabled"])
	}
	if ingressClass := settings["ingressClass"]; ingressClass != "nginx" {
		t.Fatalf("expected ingressClass to be preserved, got %#v", ingressClass)
	}
}

func TestExtractPublishAPISettingsFromMC_PrefersEnabledOverLegacyEnable(t *testing.T) {
	t.Parallel()

	moduleConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"settings": map[string]interface{}{
					"publishAPI": map[string]interface{}{
						"enable":  false,
						"enabled": true,
					},
				},
			},
		},
	}

	settings, exists, err := extractPublishAPISettingsFromMC(moduleConfig)
	if err != nil {
		t.Fatalf("extractPublishAPISettingsFromMC returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected publishAPI settings to exist")
	}
	if legacyValue, hasLegacy := settings["enable"]; hasLegacy {
		t.Fatalf("expected legacy enable key to be removed, got %#v", legacyValue)
	}
	if enabledValue := settings["enabled"]; enabledValue != true {
		t.Fatalf("expected enabled=true to win over legacy key, got %#v", enabledValue)
	}
}
