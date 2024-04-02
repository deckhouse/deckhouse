// Copyright 2021 Flant JSC
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
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExecuteTemplate(t *testing.T) {
	var data map[string]interface{}

	err := yaml.Unmarshal([]byte(`
nodeIP: "127.0.0.1"
runType: "ClusterBootstrap"
clusterConfiguration:
  kubernetesVersion: "1.29"
  clusterType: "Cloud"
  serviceSubnetCIDR: "127.0.0.1/24"
  podSubnetCIDR: "127.0.0.1/24"
  clusterDomain: "%s.example.com"
k8s:
  '1.29':
    patch: 1
extraArgs: {}
`), &data)
	if err != nil {
		t.Errorf("Loading templates error: %v", err)
	}

	_, err = RenderTemplatesDir("/deckhouse/candi/control-plane-kubeadm/", data, nil)
	if err != nil {
		t.Errorf("Rendering templates error: %v", err)
	}
}

func TestExecuteTemplate_DefineAndInclude(t *testing.T) {
	var data map[string]interface{}

	err := yaml.Unmarshal([]byte(`
nodeIP: "127.0.0.1"
`), &data)
	if err != nil {
		t.Errorf("Loading templates error: %v", err)
	}

	rendered, err := RenderTemplatesDir("testdata/execute", data, nil)
	if err != nil {
		t.Errorf("Rendering templates error: %v", err)
	}
	if len(rendered) == 0 {
		t.Errorf("Should render a template, got 0 rendered templates")
	}
	content := rendered[0].Content.String()
	// Because of a bug in templates, we have to make include and define return "NotImplemented" string
	if !strings.Contains(content, "NotImplemented" /*It should return "DEFINE GOT 127.0.0.1"*/) {
		t.Errorf("Define and include should not work in templates, got '%s'", content)
	}
}
