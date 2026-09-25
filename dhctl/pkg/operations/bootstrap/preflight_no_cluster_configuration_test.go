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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
)

// dhctl is used on a cluster whose control plane it did not create (a managed one, EKS) to install
// Deckhouse and create resources from a config that carries no ClusterConfiguration - see
// testing/cloud_layouts/EKS/WithoutNAT/configuration.tpl.yaml.
//
// The validation that used to be asserted on here — cidr-intersection and public-domain-template
// as preflight checks — is part of loading the configuration now, and tolerates the absence of a
// ClusterConfiguration there (pkg/config/cluster_network_validation_test.go). What is left to
// guard in this package is that the global suite, which runs on every bootstrap before any gate
// could exclude it, assembles for such a cluster and asks nothing of the documents it does not
// have. The case is duplicated here, away from the checks it exercises, because
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

	built := suites.NewGlobalSuite(suites.GlobalDeps{MetaConfig: metaConfig}).Checks()
	require.NotEmpty(t, built, "the global suite is what applies to a cluster with no ClusterConfiguration")

	for _, check := range built {
		require.Equal(t, preflight.PhasePreInfra, check.Phase,
			"check %q is in the global suite and must run before any infrastructure exists", check.Name)
		require.NotNil(t, check.Run, "check %q has no body", check.Name)
	}
}
