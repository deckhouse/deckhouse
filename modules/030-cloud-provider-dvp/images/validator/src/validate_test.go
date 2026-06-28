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
	"encoding/base64"
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/meta"
)

func testProviderClusterConfig(kubeconfigDataBase64 string) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "DVPClusterConfiguration",
		"provider": map[string]any{
			"kubeconfigDataBase64": kubeconfigDataBase64,
		},
	}
}

func testModuleSettings() map[string]any {
	return map[string]any{
		"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
		"storage":  map[string]any{"disabled": false, "parameters": map[string]any{}},
		"nodes":    map[string]any{"disabled": true},
	}
}

func testCredentialSecretObject() map[string]any {
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: test-token
`

	return map[string]any{
		"metadata": map[string]any{
			"name":      cpapi.CredentialSecretName,
			"namespace": dvpmeta.Namespace,
		},
		"type": cpapi.CredentialsSecretType,
		"stringData": map[string]any{
			"authScheme": string(cpapi.AuthSchemeKubeconfig),
			"secret":     base64.StdEncoding.EncodeToString([]byte(kubeconfig)),
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
		"master": testNodeGroup("master", map[string]any{"kind": dvpmeta.InstanceClassKind, "name": "master-dvp"}),
	}
	validInstanceClasses := map[string]map[string]any{
		"master-dvp": testInstanceClass("master-dvp", map[string]any{"etcdDisk": map[string]any{"size": "5Gi"}}),
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
			name: "invalid auth scheme",
			secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: func() map[string]any {
					secret := testCredentialSecretObject()
					secret["stringData"].(map[string]any)["authScheme"] = string(cpapi.AuthSchemeAPIToken)
					return secret
				}(),
			},
			nodeGroups:      validNodeGroups,
			instanceClasses: validInstanceClasses,
			want:            `Secret/d8-credentials.data.authScheme: authScheme "apiToken" is not allowed`,
		},
		{
			name: "invalid kubeconfig secret",
			secrets: map[string]map[string]any{
				cpapi.CredentialSecretName: func() map[string]any {
					secret := testCredentialSecretObject()
					secret["stringData"].(map[string]any)["secret"] = "not-base64!!!"
					return secret
				}(),
			},
			nodeGroups:      validNodeGroups,
			instanceClasses: validInstanceClasses,
			want:            `Secret/d8-credentials.data.secret: secret must contain base64-encoded kubeconfig`,
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
			name:            "master without class reference",
			secrets:         validSecrets,
			nodeGroups:      map[string]map[string]any{"master": testNodeGroup("master", nil)},
			instanceClasses: validInstanceClasses,
			want:            `NodeGroup/master.spec.cloudInstances: NodeGroup "master" with CloudPermanent nodeType must have spec.cloudInstances.classReference configured`,
		},
		{
			name:            "master instance class missing etcd disk",
			secrets:         validSecrets,
			nodeGroups:      validNodeGroups,
			instanceClasses: map[string]map[string]any{"master-dvp": testInstanceClass("master-dvp", map[string]any{})},
			want:            `DVPInstanceClass/master-dvp.spec.etcdDisk: DVPInstanceClass for NodeGroup master must define spec.etcdDisk`,
		},
		{
			name:    "worker etcd disk",
			secrets: validSecrets,
			nodeGroups: map[string]map[string]any{
				"master": validNodeGroups["master"],
				"worker": testNodeGroup("worker", map[string]any{"kind": dvpmeta.InstanceClassKind, "name": "worker"}),
			},
			instanceClasses: map[string]map[string]any{
				"master-dvp": testInstanceClass("master-dvp", map[string]any{"etcdDisk": map[string]any{"size": "5Gi"}}),
				"worker":     testInstanceClass("worker", map[string]any{"etcdDisk": map[string]any{"size": "5Gi"}}),
			},
			want: `DVPInstanceClass/worker.spec.etcdDisk: InstanceClass.spec.etcdDisk can be used only when class is attached to NodeGroup master`,
		},
		{
			name:                  "invalid provider cluster configuration kubeconfig",
			secrets:               validSecrets,
			nodeGroups:            validNodeGroups,
			instanceClasses:       validInstanceClasses,
			providerClusterConfig: testProviderClusterConfig(base64.StdEncoding.EncodeToString([]byte("not a kubeconfig"))),
			want:                  `ProviderClusterConfiguration.provider.kubeconfigDataBase64: must contain base64-encoded kubeconfig`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validate(context.Background(), proto.PrepareInput{
				Operation: proto.OperationBootstrap,
				Vars: &proto.CloudProviderVars{
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

	err := validate(context.Background(), proto.PrepareInput{
		Operation: proto.OperationBootstrap,
		Vars: &proto.CloudProviderVars{
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

func TestValidateRejectsInvalidPCCKubeconfigDuringMigration(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.PrepareInput{
		Operation:             proto.OperationBootstrap,
		ProviderClusterConfig: testProviderClusterConfig("%%%-not-base64"),
	})
	if err == nil {
		t.Fatal("validate() error = nil, want invalid PCC kubeconfig")
	}
	if !strings.Contains(err.Error(), `ProviderClusterConfiguration.provider.kubeconfigDataBase64: must contain base64-encoded kubeconfig`) {
		t.Fatalf("validate() error = %q, want PCC kubeconfig error", err)
	}
}

func TestValidateConvergeRunsPreflight(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.PrepareInput{
		Operation: proto.OperationConverge,
		Vars: &proto.CloudProviderVars{
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

	err := validate(context.Background(), proto.PrepareInput{
		Operation: proto.OperationDestroy,
		Vars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("validate() error = %v, want nil for destroy", err)
	}
}

func TestPrepareKeepsProviderVars(t *testing.T) {
	t.Parallel()

	vars := &proto.CloudProviderVars{
		Settings: map[string]any{
			"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
		},
		NodeGroups: map[string]map[string]any{
			"worker": {
				"metadata": map[string]any{"name": "worker"},
				"spec":     map[string]any{"nodeType": "CloudPermanent"},
			},
		},
	}

	result, err := prepare(context.Background(), proto.PrepareInput{Vars: vars})
	if err != nil {
		t.Fatalf("prepare() error = %v", err)
	}
	if result == nil || result.Vars == nil {
		t.Fatalf("prepare() returned nil vars")
	}
	if _, ok := result.Vars.NodeGroups["worker"]; !ok {
		t.Fatalf("prepare() expected worker NodeGroup")
	}
}
