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

package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// A cloud master is named by the infrastructure, and converge finds its machine
// again by that name — the state lives in a Secret called
// d8-node-terraform-state-<node name>. Letting --node-name through there would
// produce a master converge cannot account for, so the refusal has to come before
// anything is written to the machine.
func TestRequestNodeNameRefusesACloudCluster(t *testing.T) {
	cfg := &config.MetaConfig{ClusterType: config.CloudClusterType}

	err := requestNodeName(context.Background(), nil, cfg, "master-of-my-own")
	if err == nil {
		t.Fatal("a cloud cluster accepted --node-name")
	}
	if !strings.Contains(err.Error(), "static or hybrid") {
		t.Fatalf("the error should say where the option does apply, got: %v", err)
	}
}

// Without the option nothing is written and nothing is refused, so a cloud
// bootstrap is untouched by any of this.
func TestRequestNodeNameIsANoOpWithoutAName(t *testing.T) {
	for _, clusterType := range []string{config.CloudClusterType, config.StaticClusterType} {
		cfg := &config.MetaConfig{ClusterType: clusterType}
		// A nil node interface would panic the moment the function tried to use
		// it, so reaching the end proves it did not.
		if err := requestNodeName(context.Background(), nil, cfg, ""); err != nil {
			t.Fatalf("%s: %v", clusterType, err)
		}
	}
}
