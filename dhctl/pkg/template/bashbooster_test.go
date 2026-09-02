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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

var clusterConfig = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.32"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
proxy:
  httpProxy: http://10.130.0.31:8888
  httpsProxy: http://10.130.0.31:8888
`

var initConfig = `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
   imagesRepo: test
   devBranch: test
   # {"auths": { "test": {}}}
   registryDockerCfg: eyJhdXRocyI6IHsgInRlc3QiOiB7fX19
`

func TestRenderBashBooster(t *testing.T) {
	metaConfig, err := config.ParseConfigFromData(t.Context(), clusterConfig+initConfig, config.DummyValidatorProvider(), nil)
	if err != nil {
		t.Errorf("ParseConfigFromData error: %v", err)
	}
	mingetPath := filepath.Join(t.TempDir(), "minget")
	if err := os.WriteFile(mingetPath, []byte("test-minget"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)

	bashibleData, err := metaConfig.ConfigForBashibleBundleTemplate(t.Context(), "10.0.0.2")
	if err != nil {
		t.Fatalf("ConfigForBashibleBundleTemplate error: %v", err)
	}
	data, err := RenderBashBooster("/deckhouse/candi/bashible/bashbooster/", bashibleData)
	if err != nil {
		t.Errorf("Rendering bash booster error: %v", err)
	}

	expectedString := `export HTTP_PROXY='http://10.130.0.31:8888'`
	if !strings.Contains(data, expectedString) {
		t.Errorf("Expected string not found in data: %q", expectedString)
	}
}

func TestRenderBashBoosterEscapesProxyShellMetacharacters(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "proxy-command-executed")
	proxy := "http://user:pa'ss$(touch$IFS" + markerPath + ")@proxy.example"

	templatePath := filepath.Join("..", "..", "..", "candi", "bashible", "bashbooster", "61_proxy.sh.tpl")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	data := map[string]any{
		"proxy": map[string]any{
			"httpProxy": proxy,
		},
		"Values": map[string]any{
			"global": map[string]any{
				"clusterConfiguration": map[string]any{
					"clusterDomain":     "cluster.local",
					"podSubnetCIDR":     "10.112.0.0/16",
					"serviceSubnetCIDR": "10.223.0.0/16",
				},
			},
		},
	}

	rendered, err := RenderTemplate("61_proxy.sh.tpl", templateContent, data)
	if err != nil {
		t.Fatalf("RenderTemplate error: %v", err)
	}

	expectedString := "export HTTP_PROXY='http://user:pa'\"'\"'ss$(touch$IFS" + markerPath + ")@proxy.example'"
	if !strings.Contains(rendered.Content.String(), expectedString) {
		t.Fatalf("Expected shell-escaped proxy not found in data: %q", expectedString)
	}

	command := rendered.Content.String() + "\nbb-set-proxy\n"
	if output, err := exec.Command("bash", "-c", command).CombinedOutput(); err != nil {
		t.Fatalf("Execute rendered proxy function: %v: %s", err, output)
	}

	if _, err := os.Stat(markerPath); err == nil {
		t.Fatal("proxy value was executed as a shell command")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat marker file: %v", err)
	}
}
