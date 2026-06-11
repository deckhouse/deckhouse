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
	"fmt"
	"strings"
	"testing"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

func testModuleConfigObject(settings map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": dvpval.ModuleName,
		},
		"spec": map[string]any{
			"enabled":  true,
			"version":  2,
			"settings": settings,
		},
	}
}

func testCredentialSecretYAML() string {
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
	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: %s
`, base64.StdEncoding.EncodeToString([]byte(kubeconfig)))
}

func TestValidateBootstrapRequiresCredentialSecretOnce(t *testing.T) {
	t.Parallel()

	err := validate(context.Background(), proto.PrepareInput{
		Operation: proto.OperationBootstrap,
		ModuleConfig: testModuleConfigObject(map[string]any{
			"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
			"storage":  map[string]any{"enabled": true, "parameters": map[string]any{}},
			"nodes":    map[string]any{"enabled": false},
		}),
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
`,
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
		ModuleConfig: testModuleConfigObject(map[string]any{
			"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
			"storage":  map[string]any{"enabled": false},
			"nodes":    map[string]any{"enabled": false},
		}),
		ResourcesYAML: testCredentialSecretYAML(),
	})
	if err == nil || !strings.Contains(err.Error(), "NodeGroup \"master\" is required") {
		t.Fatalf("validate() error = %v, want master NodeGroup preflight error", err)
	}
}

func TestPrepareKeepsProviderVars(t *testing.T) {
	t.Parallel()

	result, err := prepare(context.Background(), proto.PrepareInput{
		ModuleConfig: testModuleConfigObject(map[string]any{
			"provider": map[string]any{"parameters": map[string]any{"namespace": "default"}},
		}),
		ResourcesYAML: `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudPermanent
`,
	})
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
