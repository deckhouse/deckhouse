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
	"context"
	"strings"
	"testing"

	config "github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

var clusterConfig = `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.29"
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
	metaConfig, err := config.ParseConfigFromData(context.TODO(), clusterConfig+initConfig, config.DummyPreparatorProvider())
	if err != nil {
		t.Errorf("ParseConfigFromData error: %v", err)
	}

	bashibleData, _ := metaConfig.ConfigForBashibleBundleTemplate("10.0.0.2")
	data, err := RenderBashBooster("/deckhouse/candi/bashible/bashbooster/", bashibleData)
	if err != nil {
		t.Errorf("Rendering bash booster error: %v", err)
	}

	expectedString := `export HTTP_PROXY="http://10.130.0.31:8888"`
	if !strings.Contains(data, expectedString) {
		t.Errorf("Expected string not found in data: %q", expectedString)
	}
}
