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

package dhctlproviderprotocol

import "testing"

func TestParseResourcesYAML(t *testing.T) {
	t.Parallel()

	vars, err := ParseResourcesYAML(`
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudPermanent
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ephemeral
spec:
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: worker-dvp
spec: {}
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
type: cloud-provider.deckhouse.io/credentials
---
apiVersion: v1
kind: Secret
metadata:
  name: ordinary
type: Opaque
`)
	if err != nil {
		t.Fatalf("ParseResourcesYAML() error = %v", err)
	}

	if _, ok := vars.NodeGroups["worker"]; !ok {
		t.Fatalf("ParseResourcesYAML() expected CloudPermanent NodeGroup")
	}
	if _, ok := vars.NodeGroups["ephemeral"]; ok {
		t.Fatalf("ParseResourcesYAML() must ignore non-CloudPermanent NodeGroup")
	}
	if _, ok := vars.InstanceClasses["worker-dvp"]; !ok {
		t.Fatalf("ParseResourcesYAML() expected DVPInstanceClass")
	}
	if _, ok := vars.Secrets["d8-credentials"]; !ok {
		t.Fatalf("ParseResourcesYAML() expected credential Secret")
	}
	if _, ok := vars.Secrets["ordinary"]; ok {
		t.Fatalf("ParseResourcesYAML() must ignore non-credential Secret")
	}
}

func TestParseResourcesYAMLEmpty(t *testing.T) {
	t.Parallel()

	vars, err := ParseResourcesYAML(" \n ")
	if err != nil {
		t.Fatalf("ParseResourcesYAML() error = %v", err)
	}
	if vars == nil {
		t.Fatalf("ParseResourcesYAML() returned nil vars")
	}
	if len(vars.NodeGroups) != 0 || len(vars.InstanceClasses) != 0 || len(vars.Secrets) != 0 {
		t.Fatalf("ParseResourcesYAML() expected empty vars, got %#v", vars)
	}
}
