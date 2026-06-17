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

package validation

import (
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestDecodeCredentialSecretsNilVars(t *testing.T) {
	t.Parallel()

	secrets, err := DecodeCredentialSecrets(nil)
	if err != nil {
		t.Fatalf("DecodeCredentialSecrets(nil) error = %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("DecodeCredentialSecrets(nil) = %#v, want empty slice", secrets)
	}
}

func TestDecodeNodeGroupsNilVars(t *testing.T) {
	t.Parallel()

	nodeGroups, err := DecodeNodeGroups(nil)
	if err != nil || len(nodeGroups) != 0 {
		t.Fatalf("DecodeNodeGroups(nil) = %#v, err = %v", nodeGroups, err)
	}
}

func TestDecodeInstanceClassesNilVars(t *testing.T) {
	t.Parallel()

	classes, err := DecodeInstanceClasses(nil)
	if err != nil || len(classes) != 0 {
		t.Fatalf("DecodeInstanceClasses(nil) = %#v, err = %v", classes, err)
	}
}

func TestDecodeModuleConfigEmptyRaw(t *testing.T) {
	t.Parallel()

	cfg, err := DecodeModuleConfig(nil)
	if err != nil || cfg != nil {
		t.Fatalf("DecodeModuleConfig(nil) = %#v, err = %v", cfg, err)
	}
}

func TestDecodeModuleConfigFullObject(t *testing.T) {
	t.Parallel()

	cfg, err := DecodeModuleConfig(map[string]any{
		"metadata": map[string]any{"name": "cloud-provider-dvp"},
		"spec": map[string]any{
			"enabled": true,
			"version": 2,
		},
	})
	if err != nil {
		t.Fatalf("DecodeModuleConfig() error = %v", err)
	}
	if cfg.Name != "cloud-provider-dvp" {
		t.Fatalf("DecodeModuleConfig() name = %q", cfg.Name)
	}
}

func TestDecodeModuleConfigSettingsMap(t *testing.T) {
	t.Parallel()

	moduleConfig, err := DecodeModuleConfigForModule("cloud-provider-dvp", map[string]any{
		"provider": map[string]any{
			"parameters": map[string]any{"namespace": "d8-cloud-provider-dvp"},
		},
	})
	if err != nil {
		t.Fatalf("DecodeModuleConfigForModule() error = %v", err)
	}
	if moduleConfig == nil {
		return
	}
	if moduleConfig.Name != "cloud-provider-dvp" {
		t.Fatalf("DecodeModuleConfigForModule() name = %q", moduleConfig.Name)
	}
	if moduleConfig.Spec.Version != 2 || moduleConfig.Spec.Enabled == nil || !*moduleConfig.Spec.Enabled {
		t.Fatalf("DecodeModuleConfigForModule() spec = %#v", moduleConfig.Spec)
	}
	if moduleConfig.Spec.Settings.Provider == nil || len(moduleConfig.Spec.Settings.Provider.Parameters) == 0 {
		t.Fatalf("DecodeModuleConfigForModule() settings = %#v", moduleConfig.Spec.Settings)
	}
}

func TestDecodeCredentialSecretsInvalidPayload(t *testing.T) {
	t.Parallel()

	_, err := DecodeCredentialSecrets(map[string]map[string]any{
		"broken": {"metadata": "not-an-object"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode secret") {
		t.Fatalf("DecodeCredentialSecrets() error = %v, want decode secret failure", err)
	}
}

func TestDecodeCredentialSecretMapsFields(t *testing.T) {
	t.Parallel()

	secret, err := DecodeCredentialSecret(map[string]any{
		"metadata": map[string]any{"name": "d8-credentials", "namespace": "d8-cloud-provider-dvp"},
		"type":     cpapi.CredentialsSecretType,
		"stringData": map[string]any{
			"authScheme": "kubeconfig",
			"secret":     "token",
		},
	})
	if err != nil {
		t.Fatalf("DecodeCredentialSecret() error = %v", err)
	}
	if secret.Name != "d8-credentials" || secret.Namespace != "d8-cloud-provider-dvp" {
		t.Fatalf("DecodeCredentialSecret() metadata = %#v", secret.ObjectMeta)
	}
	if secret.StringData.AuthScheme != cpapi.AuthSchemeKubeconfig || secret.StringData.Secret != "token" {
		t.Fatalf("DecodeCredentialSecret() stringData = %#v", secret.StringData)
	}
}

func TestDecodeNodeGroup(t *testing.T) {
	t.Parallel()

	nodeGroup, err := DecodeNodeGroup(map[string]any{
		"metadata": map[string]any{"name": "master"},
		"spec":     map[string]any{"nodeType": "CloudPermanent"},
	})
	if err != nil {
		t.Fatalf("DecodeNodeGroup() error = %v", err)
	}
	if nodeGroup.Name != "master" || nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
		t.Fatalf("DecodeNodeGroup() = %#v", nodeGroup)
	}
}

func TestDecodeNodeGroups(t *testing.T) {
	t.Parallel()

	nodeGroups, err := DecodeNodeGroups(map[string]map[string]any{
		"master": {
			"metadata": map[string]any{"name": "master"},
			"spec":     map[string]any{"nodeType": "CloudPermanent"},
		},
	})
	if err != nil {
		t.Fatalf("DecodeNodeGroups() error = %v", err)
	}
	if len(nodeGroups) != 1 || nodeGroups[0].Name != "master" {
		t.Fatalf("DecodeNodeGroups() = %#v", nodeGroups)
	}

	_, err = DecodeNodeGroups(map[string]map[string]any{
		"broken": {"spec": "invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode node group") {
		t.Fatalf("DecodeNodeGroups() error = %v, want decode failure", err)
	}
}

func TestDecodeInstanceClass(t *testing.T) {
	t.Parallel()

	instanceClass, err := DecodeInstanceClass(map[string]any{
		"metadata": map[string]any{"name": "master-dvp"},
		"kind":     "DVPInstanceClass",
		"spec": map[string]any{
			"etcdDisk": map[string]any{"size": "10Gi"},
		},
	})
	if err != nil {
		t.Fatalf("DecodeInstanceClass() error = %v", err)
	}
	if instanceClass.Name != "master-dvp" || instanceClass.Spec.EtcdDisk == nil {
		t.Fatalf("DecodeInstanceClass() = %#v", instanceClass)
	}
}

func TestDecodeInstanceClasses(t *testing.T) {
	t.Parallel()

	classes, err := DecodeInstanceClasses(map[string]map[string]any{
		"master-dvp": {
			"metadata": map[string]any{"name": "master-dvp"},
			"kind":     "DVPInstanceClass",
		},
	})
	if err != nil {
		t.Fatalf("DecodeInstanceClasses() error = %v", err)
	}
	if len(classes) != 1 || classes[0].Name != "master-dvp" {
		t.Fatalf("DecodeInstanceClasses() = %#v", classes)
	}

	_, err = DecodeInstanceClasses(map[string]map[string]any{
		"broken": {"spec": "invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode instance class") {
		t.Fatalf("DecodeInstanceClasses() error = %v, want decode failure", err)
	}
}

func TestDecodeJSONValueRoundTrip(t *testing.T) {
	t.Parallel()

	value, err := DecodeJSONValue[cpapi.NodeGroup](map[string]any{
		"metadata": map[string]any{"name": "master"},
		"spec":     map[string]any{"nodeType": "CloudPermanent"},
	})
	if err != nil {
		t.Fatalf("DecodeJSONValue() error = %v", err)
	}
	if value.Name != "master" || value.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
		t.Fatalf("DecodeJSONValue() = %#v", value)
	}
}

func TestDecodeJSONValueInvalidTarget(t *testing.T) {
	t.Parallel()

	_, err := DecodeJSONValue[int]("not-a-number")
	if err == nil || !strings.Contains(err.Error(), "unmarshal value") {
		t.Fatalf("DecodeJSONValue() error = %v, want unmarshal failure", err)
	}
}
