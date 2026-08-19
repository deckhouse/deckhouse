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

package immutable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
)

// TestClusterParamsResolvesAutomaticKubernetesVersion covers the one cluster
// setting the node cannot interpret: "Automatic" is an installer default, and
// the node would render it straight into the feature gates of every component.
func TestClusterParamsResolvesAutomaticKubernetesVersion(t *testing.T) {
	metaConfig := immutabletest.MetaConfig(t)
	metaConfig.ClusterConfig["kubernetesVersion"] = json.RawMessage(`"Automatic"`)

	params, err := clusterParams(metaConfig)
	require.NoError(t, err)
	require.Equal(t, config.DefaultKubernetesVersion, params.KubernetesVersion)
}

// TestClusterParams pins the inputs behind the component flags. Every one of
// them renders as an empty flag when it is missing, and the component then dies
// on its own command line with nothing pointing back at the configuration.
func TestClusterParams(t *testing.T) {
	metaConfig := immutabletest.MetaConfig(t)

	params, err := clusterParams(metaConfig)
	require.NoError(t, err)
	require.Equal(t, controlPlaneRenderParams{
		clusterParamsSpec: clusterParamsSpec{
			ClusterDomain:     "cluster.local",
			ServiceSubnetCIDR: "10.223.0.0/16",
		},
		PodSubnetCIDR:           "10.222.0.0/16",
		PodSubnetNodeCIDRPrefix: "24",
		KubernetesVersion:       "1.34",
		ClusterType:             config.CloudClusterType,
	}, params)

	for _, key := range []string{"clusterDomain", "serviceSubnetCIDR", "podSubnetCIDR", "podSubnetNodeCIDRPrefix"} {
		t.Run("missing "+key, func(t *testing.T) {
			metaConfig := immutabletest.MetaConfig(t)
			delete(metaConfig.ClusterConfig, key)

			_, err := clusterParams(metaConfig)
			require.ErrorContains(t, err, key+" is empty in the cluster configuration")
		})
	}
}

// TestResolveControlPlaneImages pins what the templates are handed: bare
// digests, with the registry address and path prepended at render time.
func TestResolveControlPlaneImages(t *testing.T) {
	metaConfig := immutabletest.MetaConfig(t)

	images, err := ResolveControlPlaneImages(t.Context(), metaConfig)
	require.NoError(t, err)
	require.Equal(t, controlPlaneImages{
		Etcd:                  immutabletest.EtcdDigest,
		KubeAPIServer:         immutabletest.APIServerDigest,
		KubeControllerManager: immutabletest.ControllerManagerDigest,
		KubeScheduler:         immutabletest.SchedulerDigest,
	}, images)
}

// TestResolveControlPlaneImagesMissingVersion is what the preflight check
// guards: an installer that does not ship a control plane for the requested
// minor hands the node an empty image and the static pod never starts.
func TestResolveControlPlaneImagesMissingVersion(t *testing.T) {
	metaConfig := immutabletest.MetaConfig(t)
	metaConfig.ClusterConfig["kubernetesVersion"] = json.RawMessage(`"1.30"`)

	_, err := ResolveControlPlaneImages(t.Context(), metaConfig)
	require.ErrorContains(t, err, `no "controlPlaneManager"."kubeApiserver130" image digest`)
	require.ErrorContains(t, err, "Kubernetes 1.30")
}

// TestCertSANs reads the same list control-plane-manager publishes under the
// "cert-sans" key of its config secret: a name missing here is one the
// apiserver certificate does not cover until control-plane-manager reissues it.
func TestCertSANs(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		expSANs  []string
	}{
		{
			name:     "no module config at all",
			settings: nil,
		},
		{
			name:     "module config without an apiserver section",
			settings: map[string]any{"encryptionAlgorithm": "ECDSA-P256"},
		},
		{
			name:     "the configured names",
			settings: map[string]any{"apiserver": map[string]any{"certSANs": []any{"ha.example.com", "192.168.1.100"}}},
			expSANs:  []string{"ha.example.com", "192.168.1.100"},
		},
		{
			name:     "an empty list is no list",
			settings: map[string]any{"apiserver": map[string]any{"certSANs": []any{}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := immutabletest.MetaConfig(t)
			if tt.settings != nil {
				metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, &config.ModuleConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "control-plane-manager"},
					Spec:       config.ModuleConfigSpec{Settings: tt.settings},
				})
			}

			require.Equal(t, tt.expSANs, certSANs(metaConfig))
		})
	}
}
