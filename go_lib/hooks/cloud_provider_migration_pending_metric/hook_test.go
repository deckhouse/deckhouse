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

package cloud_provider_migration_pending_metric

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// unstructuredFromObj marshals a typed object into *unstructured.Unstructured.
func unstructuredFromObj(obj interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	raw, _ := json.Marshal(obj)
	json.Unmarshal(raw, &u.Object)
	return u
}

func makeConfigMapUnstructured(name, namespace string, data map[string]string) *unstructured.Unstructured {
	return unstructuredFromObj(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	})
}

// --------------------------------------------------------------------------
// filterMigrationMarkerConfigMap
// --------------------------------------------------------------------------

func TestFilterMigrationMarkerConfigMap_Matches(t *testing.T) {
	fn := filterMigrationMarkerConfigMap("d8-module-is-migrating")
	obj := makeConfigMapUnstructured("d8-module-is-migrating", "d8-ns", nil)
	result, err := fn(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(string); !ok || s != "d8-module-is-migrating" {
		t.Fatalf("expected name, got %v", result)
	}
}

func TestFilterMigrationMarkerConfigMap_WrongName(t *testing.T) {
	fn := filterMigrationMarkerConfigMap("d8-module-is-migrating")
	obj := makeConfigMapUnstructured("other-cm", "d8-ns", nil)
	result, err := fn(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil for wrong name, got %v", result)
	}
}

func TestFilterMigrationMarkerConfigMap_DifferentName(t *testing.T) {
	fn := filterMigrationMarkerConfigMap("custom-migration-cm")
	obj := makeConfigMapUnstructured("custom-migration-cm", "ns", nil)
	result, err := fn(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(string); !ok || s != "custom-migration-cm" {
		t.Fatalf("expected custom name, got %v", result)
	}
}

// --------------------------------------------------------------------------
// filterCommanderUUIDConfigMap
// --------------------------------------------------------------------------

func TestFilterCommanderUUIDConfigMap_Matches(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderUUIDConfigMapName, commanderUUIDNamespace, nil)
	result, err := filterCommanderUUIDConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(string); !ok || s != commanderUUIDConfigMapName {
		t.Fatalf("expected name, got %v", result)
	}
}

func TestFilterCommanderUUIDConfigMap_WrongName(t *testing.T) {
	obj := makeConfigMapUnstructured("other", commanderUUIDNamespace, nil)
	result, err := filterCommanderUUIDConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestFilterCommanderUUIDConfigMap_WrongNamespace(t *testing.T) {
	// Name matches, namespace differs — filter doesn't check namespace (it's handled
	// by the NamespaceSelector on the binding), so this should return the name.
	obj := makeConfigMapUnstructured(commanderUUIDConfigMapName, "other-ns", nil)
	result, err := filterCommanderUUIDConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(string); !ok || s != commanderUUIDConfigMapName {
		t.Fatalf("expected name regardless of namespace, got %v", result)
	}
}

// --------------------------------------------------------------------------
// filterCommanderInfoConfigMap
// --------------------------------------------------------------------------

func TestFilterCommanderInfoConfigMap_WrongName(t *testing.T) {
	obj := makeConfigMapUnstructured("other", commanderInfoNamespace, nil)
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestFilterCommanderInfoConfigMap_FlagTrue(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{"flags":{"cloudProviderNoPCCInputFormatSupported":"1"}}`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if !info.Supported {
		t.Fatal("expected Supported=true when flag is '1'")
	}
}

func TestFilterCommanderInfoConfigMap_FlagFalse(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{"flags":{"cloudProviderNoPCCInputFormatSupported":"0"}}`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when flag is '0'")
	}
}

func TestFilterCommanderInfoConfigMap_FlagMissing(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{"flags":{"otherFlag":"1"}}`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when flag is absent")
	}
}

func TestFilterCommanderInfoConfigMap_EmptyData(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, nil)
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when data.json is absent")
	}
}

func TestFilterCommanderInfoConfigMap_EmptyDataJSON(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: "",
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when data.json is empty string")
	}
}

func TestFilterCommanderInfoConfigMap_InvalidJSON(t *testing.T) {
	// fail-closed: unparsable JSON → Supported=false
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{invalid`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when data.json is invalid (fail-closed)")
	}
}

func TestFilterCommanderInfoConfigMap_EmptyFlags(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{"flags":{}}`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when flags is empty")
	}
}

func TestFilterCommanderInfoConfigMap_NoFlagsKey(t *testing.T) {
	obj := makeConfigMapUnstructured(commanderInfoConfigMapName, commanderInfoNamespace, map[string]string{
		commanderInfoDataKey: `{}`,
	})
	result, err := filterCommanderInfoConfigMap(obj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info := result.(commanderInfoResult)
	if info.Supported {
		t.Fatal("expected Supported=false when flags key is absent")
	}
}

// --------------------------------------------------------------------------
// commanderInfoResult zero value
// --------------------------------------------------------------------------

func TestCommanderInfoResult_ZeroValue_NotSupported(t *testing.T) {
	r := commanderInfoResult{}
	if r.Supported {
		t.Fatal("zero-value commanderInfoResult must have Supported=false")
	}
}

// --------------------------------------------------------------------------
// commanderInfoData serialisation round-trip
// --------------------------------------------------------------------------

func TestCommanderInfoData_Unmarshal_FlagTrue(t *testing.T) {
	raw := `{"flags":{"cloudProviderNoPCCInputFormatSupported":"1"}}`
	var data commanderInfoData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if data.Flags[commanderPCCSupportFlag] != "1" {
		t.Fatalf("expected flag='1', got %q", data.Flags[commanderPCCSupportFlag])
	}
}

func TestCommanderInfoData_Unmarshal_FlagFalse(t *testing.T) {
	raw := `{"flags":{"cloudProviderNoPCCInputFormatSupported":"0"}}`
	var data commanderInfoData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if data.Flags[commanderPCCSupportFlag] != "0" {
		t.Fatalf("expected flag='0', got %q", data.Flags[commanderPCCSupportFlag])
	}
}

func TestCommanderInfoData_Unmarshal_NoFlags(t *testing.T) {
	raw := `{}`
	var data commanderInfoData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if data.Flags != nil {
		t.Fatal("expected Flags=nil for empty JSON")
	}
}
