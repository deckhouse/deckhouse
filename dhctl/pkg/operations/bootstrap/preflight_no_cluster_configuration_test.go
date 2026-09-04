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
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
)

// dhctl is used on a cluster whose control plane it did not create (a managed one, EKS) to install
// Deckhouse and create resources from a config that carries no ClusterConfiguration - see
// testing/cloud_layouts/EKS/WithoutNAT/configuration.tpl.yaml. The global suite runs on every
// bootstrap, before any gate could exclude it, so its ClusterConfiguration-reading checks must skip
// instead of refusing. The case is duplicated here, away from the checks it exercises, because
// hack/coverage.sh:20 keeps the whole /pkg/preflight tree out of CI.
func TestGlobalPreflightSuiteWithoutClusterConfiguration(t *testing.T) {
	metaConfig := &config.MetaConfig{
		ModuleConfigs: []*config.ModuleConfig{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "global"},
				Spec: config.ModuleConfigSpec{
					Settings: map[string]any{
						"modules": map[string]any{
							"publicDomainTemplate": "%s.example.com",
						},
					},
				},
			},
		},
	}

	want := []preflight.CheckName{checks.CidrIntersectionCheckName, checks.PublicDomainTemplateCheckName}
	found := 0

	for _, check := range suites.NewGlobalSuite(suites.GlobalDeps{MetaConfig: metaConfig}).Checks() {
		if !slices.Contains(want, check.Name) {
			continue
		}

		require.NoError(t, check.Run(t.Context()), "check %q must pass without a ClusterConfiguration", check.Name)

		found++
	}

	require.Equal(t, len(want), found, "a check left the global suite")
}
