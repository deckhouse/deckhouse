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
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

func testModuleSettings() map[string]any {
	return map[string]any{
		"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
		"storage":  map[string]any{"enabled": true, "parameters": map[string]any{}},
		"nodes":    map[string]any{"enabled": false},
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
			"namespace": dvpval.Namespace,
		},
		"type": cpapi.CredentialsSecretType,
		"stringData": map[string]any{
			"authScheme": string(cpapi.AuthSchemeKubeconfig),
			"secret":     base64.StdEncoding.EncodeToString([]byte(kubeconfig)),
		},
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

func TestValidateConvergeRunsPreflight(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.PrepareInput{
		Operation: proto.OperationConverge,
		Vars: &proto.CloudProviderVars{
			Settings: map[string]any{
				"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
				"storage":  map[string]any{"enabled": false},
				"nodes":    map[string]any{"enabled": false},
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
