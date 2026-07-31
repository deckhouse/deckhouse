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
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite the golden payload files")

// TestBuildCloudConfigGolden pins the exact bytes the master VM boots with. The
// on-node agent parses both documents strictly, so a silent rename of any field
// here is a node that refuses to bootstrap.
//
// The control-plane document is assembled by hand rather than through
// BuildControlPlaneConfig: the real one embeds freshly generated CA material and
// the rendered candi manifests, neither of which is byte-stable. The manifest
// rendering itself is covered by pkg/template/controlplane_manifests_test.go.
func TestBuildCloudConfigGolden(t *testing.T) {
	metaConfig := testMetaConfig(t, "50Gi", "10Gi")

	nodeConfig, err := BuildNodeConfig(context.Background(), NodeConfigInput{
		NodeName:   "zykov-master-0",
		MetaConfig: metaConfig,
	})
	require.NoError(t, err)

	_, controlPlaneDisk, err := MasterDisks(metaConfig)
	require.NoError(t, err)

	controlPlaneConfig := &ControlPlaneConfig{
		APIVersion: PayloadAPIVersion,
		Kind:       ControlPlaneConfigKind,
		Metadata:   ObjectMeta{Name: "zykov-master-0"},
		Spec: ControlPlaneSpec{
			Bootstrap: true,
			Disk:      controlPlaneDisk,
			CA: map[string]string{
				"ca.crt":             "<ca.crt>",
				"ca.key":             "<ca.key>",
				"front-proxy-ca.crt": "<front-proxy-ca.crt>",
				"front-proxy-ca.key": "<front-proxy-ca.key>",
				"etcd/ca.crt":        "<etcd/ca.crt>",
				"etcd/ca.key":        "<etcd/ca.key>",
				"sa.key":             "<sa.key>",
				"sa.pub":             "<sa.pub>",
			},
			Params: ControlPlaneParams{
				ClusterDomain:     "cluster.local",
				ServiceSubnetCIDR: "10.223.0.0/16",
			},
			ExtraFiles: map[string]string{authenticationConfigFile: authenticationConfig},
			Manifests: map[string]string{
				"etcd.yaml":                    "<etcd.yaml>\n",
				"kube-apiserver.yaml":          "<kube-apiserver.yaml>\n",
				"kube-controller-manager.yaml": "<kube-controller-manager.yaml>\n",
				"kube-scheduler.yaml":          "<kube-scheduler.yaml>\n",
			},
		},
	}

	cloudConfig, err := BuildCloudConfig(nodeConfig, controlPlaneConfig)
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "master-cloud-init.yaml")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(cloudConfig), 0o644))
	}

	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	require.Equal(t, string(golden), cloudConfig)
}

// TestBuildCloudConfigHasNoConflictingKeys guards the one cloud-init rule the
// provider's terraform imposes: it concatenates this document with a block of
// its own, so a top-level key both emit breaks the whole user-data.
func TestBuildCloudConfigHasNoConflictingKeys(t *testing.T) {
	metaConfig := testMetaConfig(t, "50Gi", "10Gi")

	nodeConfig, err := BuildNodeConfig(context.Background(), NodeConfigInput{
		NodeName:   "zykov-master-0",
		MetaConfig: metaConfig,
	})
	require.NoError(t, err)

	cloudConfig, err := BuildCloudConfig(nodeConfig, &ControlPlaneConfig{
		APIVersion: PayloadAPIVersion,
		Kind:       ControlPlaneConfigKind,
	})
	require.NoError(t, err)

	var document map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(cloudConfig), &document))

	require.Contains(t, document, "write_files")
	for _, forbidden := range []string{"hostname", "users", "ssh_authorized_keys"} {
		require.NotContains(t, document, forbidden)
	}
	require.Len(t, document, 1)
}
