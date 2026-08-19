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

// Package immutabletest holds the fixtures the immutable payload and the
// bootstrap steps that drive it are both tested against. It imports the
// immutable package on purpose not at all: that package's own tests are
// internal, and importing it back would be a cycle.
package immutabletest

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

// The digests MetaConfig carries. Distinct per image so that a test asserting on
// one of them cannot pass by picking up another.
const (
	ContainerdDigest        = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	CNIDigest               = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	KubeletDigest           = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	PauseDigest             = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	OSImageDigest           = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	NodeletDigest           = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
	EtcdDigest              = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	APIServerDigest         = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	ControllerManagerDigest = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	SchedulerDigest         = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
)

// MetaConfig is a cloud cluster whose master NodeGroup asks for an immutable
// system. The golden payload is rendered from it, so a change here rewrites
// testdata/master-cloud-init.yaml.
func MetaConfig(t *testing.T) *config.MetaConfig {
	t.Helper()

	const masterNodeGroup = `{
	  "replicas": 1,
	  "instanceClass": {
	    "rootDisk": {"size": "50Gi"},
	    "etcdDisk": {"size": "10Gi"}
	  }
	}`

	metaConfig := &config.MetaConfig{
		ClusterType:       config.CloudClusterType,
		ClusterPrefix:     "example",
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
			"masterNodeGroup": json.RawMessage(masterNodeGroup),
		},
		// What config.Prepare parses the resources section into, and the only
		// place the immutable path is asked for.
		CloudProviderVars: &config.CloudProviderVars{
			NodeGroups: map[string]map[string]any{
				"master": {
					"apiVersion": "deckhouse.io/v1",
					"kind":       "NodeGroup",
					"spec":       map[string]any{"nodeType": "CloudPermanent", "systemType": "Immutable"},
				},
			},
		},
		Images: map[string]map[string]any{
			"registrypackages": {
				"containerdSysext224":    ContainerdDigest,
				"kubernetesCniSysext162": CNIDigest,
				"kubeletSysext1349":      KubeletDigest,
				"kubeletSysext1336":      "sha256:0000000000000000000000000000000000000000000000000000000000000000",
				"nodeletSysext":          NodeletDigest,
			},
			"nodeManager": {
				"olcedar": OSImageDigest,
			},
			"common": {
				"pause": PauseDigest,
			},
			"controlPlaneManager": {
				"etcd":                     EtcdDigest,
				"kubeApiserver134":         APIServerDigest,
				"kubeControllerManager134": ControllerManagerDigest,
				"kubeScheduler134":         SchedulerDigest,
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

// CandiDir finds the checkout's own candi directory by walking up from the
// test's working directory, so the render reads the same templates the classic
// bootstrap renders. The marker is immutable.controlPlaneTemplatesDir.
func CandiDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		candidate := filepath.Join(dir, "candi")
		if _, err := os.Stat(filepath.Join(candidate, "control-plane")); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "candi/control-plane not found above %s", dir)
		dir = parent
	}
}

// HandoffServer serves the node's side of the bootstrap channel over the
// certificate the payload carried, the way the node does. It takes the PEMs
// rather than the material so that this package needs no import of immutable.
func HandoffServer(t *testing.T, serverCertPEM, serverKeyPEM string, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	certificate, err := tls.X509KeyPair([]byte(serverCertPEM), []byte(serverKeyPEM))
	require.NoError(t, err, "the node parses the payload certificate with exactly this call")

	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	t.Cleanup(server.Close)

	return server
}
