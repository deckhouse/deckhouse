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
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// TestBuildImmutableMasterPayloadIsBase64CloudConfig pins the contract with the
// consumer of the payload: it travels in the "cloudConfig" tfvar, which every
// provider's terraform base64decodes before writing the cloud-init secret. A
// plain document there fails terraform at apply time, after the base
// infrastructure already exists.
func TestBuildImmutableMasterPayloadIsBase64CloudConfig(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)

	payload, err := b.buildImmutableMasterPayload(t.Context(), bctx, "zykov-master-0")
	require.NoError(t, err)

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err, "the payload must be base64: terraform base64decodes it")

	require.True(t, strings.HasPrefix(string(document), "#cloud-config\n"), "the decoded payload must be a cloud-config document")

	var cloudConfig struct {
		WriteFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"write_files"`
	}
	require.NoError(t, yaml.Unmarshal(document, &cloudConfig))

	files := make(map[string]string, len(cloudConfig.WriteFiles))
	for _, file := range cloudConfig.WriteFiles {
		files[file.Path] = file.Content
	}
	require.Contains(t, files, "/config/nodeconfig.yaml")
	require.Contains(t, files, "/config/controlplane.yaml")

	// Both documents are parsed on the node, so they have to survive the round
	// trip as YAML rather than as an opaque blob.
	var nodeConfig, controlPlaneConfig map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(files["/config/nodeconfig.yaml"]), &nodeConfig))
	require.NoError(t, yaml.Unmarshal([]byte(files["/config/controlplane.yaml"]), &controlPlaneConfig))
	require.Equal(t, "NodeConfig", nodeConfig["kind"])
	require.Equal(t, "ControlPlaneConfig", controlPlaneConfig["kind"])
}

// immutableTestBootstrapper builds the smallest bootstrapper that can render
// the master payload.
func immutableTestBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext) {
	t.Helper()

	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	b := &ClusterBootstrapper{Params: &Params{Options: options.New()}}

	return b, &bootstrapContext{
		metaConfig: immutableTestMetaConfig(t),
		stateCache: stateCache,
	}
}

func immutableTestMetaConfig(t *testing.T) *config.MetaConfig {
	t.Helper()

	const digest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

	metaConfig := &config.MetaConfig{
		ClusterType:       config.CloudClusterType,
		ClusterPrefix:     "zykov",
		ClusterDomain:     "cluster.local",
		ClusterDNSAddress: "10.223.0.10",
		ClusterConfig: map[string]json.RawMessage{
			"kubernetesVersion":       json.RawMessage(`"1.34"`),
			"serviceSubnetCIDR":       json.RawMessage(`"10.223.0.0/16"`),
			"podSubnetCIDR":           json.RawMessage(`"10.222.0.0/16"`),
			"podSubnetNodeCIDRPrefix": json.RawMessage(`"24"`),
			"clusterDomain":           json.RawMessage(`"cluster.local"`),
		},
		ProviderClusterConfig: map[string]json.RawMessage{
			"masterNodeGroup": json.RawMessage(`{
			  "replicas": 1,
			  "instanceClass": {
			    "rootDisk": {"size": "50Gi"},
			    "etcdDisk": {"size": "10Gi"}
			  }
			}`),
		},
		Images: map[string]map[string]any{
			"registrypackages": {
				"containerdSysext224":    digest,
				"kubernetesCniSysext162": digest,
				"kubeletSysext1349":      digest,
			},
			"common": {"pause": digest},
			"controlPlaneManager": {
				"etcd":                     digest,
				"kubeApiserver134":         digest,
				"kubeControllerManager134": digest,
				"kubeScheduler134":         digest,
			},
		},
	}

	metaConfig.Registry.Settings = registry.ModeSettings{
		Mode: constant.ModeUnmanaged,
		RemoteData: registry.Data{
			ImagesRepo: "dev-registry.deckhouse.io/sys/deckhouse-oss",
			Scheme:     constant.SchemeHTTPS,
			Username:   "user",
			Password:   "password",
		},
	}

	return metaConfig
}
