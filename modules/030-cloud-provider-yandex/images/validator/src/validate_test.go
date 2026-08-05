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

package main

import (
	"context"
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

// testProviderClusterConfig returns a legacy YandexClusterConfiguration with the given layout.
// Note the legacy spelling etcdDiskSizeGb, which differs from etcdDiskSizeGB in the CRD.
func testProviderClusterConfig(layout string, masterNodeGroup map[string]any) map[string]any {
	pcc := map[string]any{
		"apiVersion":      "deckhouse.io/v1",
		"kind":            "YandexClusterConfiguration",
		"layout":          layout,
		"sshPublicKey":    "ssh-rsa test",
		"nodeNetworkCIDR": "10.100.0.0/21",
		"masterNodeGroup": masterNodeGroup,
		"provider": map[string]any{
			"cloudID":            "test",
			"folderID":           "test",
			"serviceAccountJSON": `{"id": "test"}`,
		},
	}

	return pcc
}

func testMasterNodeGroup(replicas int, externalIPAddresses []string) map[string]any {
	instanceClass := map[string]any{
		"cores":          4,
		"memory":         8192,
		"imageID":        "test",
		"etcdDiskSizeGb": 10,
	}
	if externalIPAddresses != nil {
		instanceClass["externalIPAddresses"] = externalIPAddresses
	}

	return map[string]any{
		"replicas":      replicas,
		"instanceClass": instanceClass,
	}
}

func testModuleSettings() map[string]any {
	return map[string]any{
		"provider": map[string]any{
			"parameters": map[string]any{
				"cloudID":      "test",
				"folderID":     "test",
				"sshPublicKey": "ssh-rsa test",
			},
		},
		"nodes": map[string]any{
			"parameters": map[string]any{
				"layout":          "Standard",
				"nodeNetworkCIDR": "10.100.0.0/21",
			},
		},
		"storage": map[string]any{"disabled": false, "parameters": map[string]any{}},
	}
}

// testCredentialSecretObject returns a valid Yandex credential Secret: the provider supports
// the serviceAccount (IAM key JSON) and apiToken auth schemes, see ycval.CredentialsValidator.
func testCredentialSecretObject() map[string]any {
	serviceAccountJSON := `{"id": "test", "service_account_id": "test"}`

	return map[string]any{
		"metadata": map[string]any{
			"name":      cpapi.CredentialSecretName,
			"namespace": ycmeta.Namespace,
		},
		"type": cpapi.CredentialsSecretType,
		"stringData": map[string]any{
			"authScheme": string(cpapi.AuthSchemeServiceAccount),
			"secret":     serviceAccountJSON,
		},
	}
}

func testNodeGroup(name string, classReference map[string]any) map[string]any {
	spec := map[string]any{"nodeType": "CloudPermanent"}
	if classReference != nil {
		spec["cloudInstances"] = map[string]any{"classReference": classReference}
	}

	return map[string]any{
		"metadata": map[string]any{"name": name},
		"spec":     spec,
	}
}

func testInstanceClass(name string, spec map[string]any) map[string]any {
	return map[string]any{
		"metadata": map[string]any{"name": name},
		"spec":     spec,
	}
}

func TestValidateMatchesDhctlBootstrapFailures(t *testing.T) {
	t.Parallel()

	validSecrets := map[string]map[string]any{cpapi.CredentialSecretName: testCredentialSecretObject()}
	validNodeGroups := map[string]map[string]any{
		"master": testNodeGroup("master", map[string]any{"kind": "YandexInstanceClass", "name": "master-yandex"}),
	}
	validInstanceClasses := map[string]map[string]any{
		"master-yandex": testInstanceClass("master-yandex", map[string]any{"etcdDiskSizeGB": 10}),
	}

	tests := []struct {
		name                  string
		secrets               map[string]map[string]any
		nodeGroups            map[string]map[string]any
		instanceClasses       map[string]map[string]any
		providerClusterConfig map[string]any
		want                  string
	}{
		{
			name:            "missing d8 credentials",
			nodeGroups:      validNodeGroups,
			instanceClasses: validInstanceClasses,
			want:            `Secret/d8-credentials: credential Secret "d8-credentials" is required`,
		},
		{
			// Yandex accepts only serviceAccount and apiToken.
			name: "invalid auth scheme",
			secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: func() map[string]any {
					secret := testCredentialSecretObject()
					secret["stringData"].(map[string]any)["authScheme"] = string(cpapi.AuthSchemeKubeconfig)
					return secret
				}(),
			},
			nodeGroups:      validNodeGroups,
			instanceClasses: validInstanceClasses,
			want:            `Secret/d8-credentials.data.authScheme: authScheme "kubeconfig" is not allowed`,
		},
		{
			name: "invalid service account secret",
			secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: func() map[string]any {
					secret := testCredentialSecretObject()
					secret["stringData"].(map[string]any)["secret"] = "not-json!!!"
					return secret
				}(),
			},
			nodeGroups:      validNodeGroups,
			instanceClasses: validInstanceClasses,
			want:            `Secret/d8-credentials.data.secret: invalid service account`,
		},
		{
			name:    "missing master nodegroup",
			secrets: validSecrets,
			nodeGroups: map[string]map[string]any{
				"worker": {"metadata": map[string]any{"name": "worker"}, "spec": map[string]any{"nodeType": "CloudPermanent"}},
			},
			instanceClasses: validInstanceClasses,
			want:            `NodeGroup/master: NodeGroup "master" is required`,
		},
		{
			name:            "master instance class missing etcd disk",
			secrets:         validSecrets,
			nodeGroups:      validNodeGroups,
			instanceClasses: map[string]map[string]any{"master-yandex": testInstanceClass("master-yandex", map[string]any{})},
			want:            `YandexInstanceClass/master-yandex.spec.etcdDisk: YandexInstanceClass for NodeGroup master must define spec.etcdDisk`,
		},
		{
			name:    "worker etcd disk",
			secrets: validSecrets,
			nodeGroups: map[string]map[string]any{
				"master": validNodeGroups["master"],
				"worker": testNodeGroup("worker", map[string]any{"kind": "YandexInstanceClass", "name": "worker"}),
			},
			instanceClasses: map[string]map[string]any{
				"master-yandex": testInstanceClass("master-yandex", map[string]any{"etcdDiskSizeGB": 10}),
				"worker":        testInstanceClass("worker", map[string]any{"etcdDiskSizeGB": 10}),
			},
			want: `YandexInstanceClass/worker.spec.etcdDisk: InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master`,
		},
		{
			name:                  "legacy PCC master replicas exceed external IP addresses",
			secrets:               validSecrets,
			nodeGroups:            validNodeGroups,
			instanceClasses:       validInstanceClasses,
			providerClusterConfig: testProviderClusterConfig("Standard", testMasterNodeGroup(2, []string{"1.2.3.4"})),
			want:                  `ProviderClusterConfiguration.masterNodeGroup.instanceClass.externalIPAddresses: number of masterNodeGroup.replicas (2) should be less or equal to the length of masterNodeGroup.instanceClass.externalIPAddresses (1)`,
		},
		{
			name:                  "legacy PCC WithNATInstance without internal subnet",
			secrets:               validSecrets,
			nodeGroups:            validNodeGroups,
			instanceClasses:       validInstanceClasses,
			providerClusterConfig: testProviderClusterConfig("WithNATInstance", testMasterNodeGroup(1, nil)),
			want:                  `ProviderClusterConfiguration.withNATInstance: must provide internalSubnetCIDR or internalSubnetID for withNATInstance`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate(context.Background(), proto.ValidateInput{
				Operation: proto.OperationBootstrap,
				CloudProviderVars: &proto.CloudProviderVars{
					Settings:        testModuleSettings(),
					Secrets:         tt.secrets,
					NodeGroups:      tt.nodeGroups,
					InstanceClasses: tt.instanceClasses,
				},
				ProviderClusterConfig: tt.providerClusterConfig,
			})
			if err == nil {
				t.Fatalf("validate() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validate() error = %q, want to contain %q", err, tt.want)
			}
		})
	}
}

func TestValidateBootstrapRequiresCredentialSecretOnce(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.ValidateInput{
		Operation: proto.OperationBootstrap,
		CloudProviderVars: &proto.CloudProviderVars{
			Settings: testModuleSettings(),
			NodeGroups: map[string]map[string]any{
				"master": {
					"metadata": map[string]any{"name": "master"},
					"spec":     map[string]any{"nodeType": "CloudPermanent"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("validate() error = nil, want missing credential secret")
	}
	if strings.Count(err.Error(), `credential Secret "d8-credentials" is required`) != 1 {
		t.Fatalf("validate() error = %q, want single credential requirement message", err)
	}
}

func TestValidateConvergeRunsPreflight(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.ValidateInput{
		Operation: proto.OperationConverge,
		CloudProviderVars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
				"storage":  map[string]any{"disabled": true},
				"nodes":    map[string]any{"disabled": true},
			},
			Secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: testCredentialSecretObject(),
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), `NodeGroup "master" is required`) {
		t.Fatalf("validate() error = %v, want master NodeGroup preflight error", err)
	}
}

func TestValidateDestroySkipsValidation(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.ValidateInput{
		Operation: proto.OperationDestroy,
		CloudProviderVars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("validate() error = %v, want nil for destroy", err)
	}
}
