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
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
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

// TestAdminKubeconfigFromCache covers the rerun. The bootstrap has phases left
// after the handover, and nothing skips a phase that already completed — so a
// rerun re-enters this step with the node's bootstrap channel closing or already
// shut. It has to read the credentials the first attempt saved instead of
// waiting half an hour on a listener that is gone.
func TestAdminKubeconfigFromCache(t *testing.T) {
	const collected = "apiVersion: v1\nkind: Config\n"

	t.Run("nothing has been collected yet", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err)
		require.Nil(t, content, "with no record there is nothing to reuse; the handoff channel is the only source")
		require.Empty(t, path)
	})

	t.Run("an earlier attempt collected them", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		saved := filepath.Join(t.TempDir(), "zykov-admin.kubeconfig")
		require.NoError(t, os.WriteFile(saved, []byte(collected), 0o600))
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, saved))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err)
		require.Equal(t, collected, string(content))
		require.Equal(t, saved, path)
	})

	// The record is written before the handover is confirmed, so it does not say
	// the node's channel is closed — and the operator is told this file is the
	// only way in, which invites moving it. Refusing here would turn that into a
	// bootstrap no rerun can finish, with no flag to clear the record.
	t.Run("the recorded file has been moved away", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		missing := filepath.Join(t.TempDir(), "zykov-admin.kubeconfig")
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, missing))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err, "an unreadable record must fall through to the node, not end the bootstrap")
		require.Nil(t, content)
		require.Empty(t, path)
	})
}

// The record has to be written before ConfirmCollected, which is what makes the
// node shut its channel for good. saveAdminKubeconfig is the last point where
// that ordering still holds, so the record is written there — and a rerun that
// died anywhere after it finds the file rather than a dead channel.
func TestSaveAdminKubeconfigRecordsThePathBeforeTheChannelCloses(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	recorded, err := immutable.LoadCollectedKubeconfig(t.Context(), bctx.stateCache)
	require.NoError(t, err)
	require.Equal(t, bctx.adminKubeconfigPath, recorded,
		"the rerun path must be usable from the moment the file exists, not from the confirmation")
}

// TestSaveAdminKubeconfigNamesTheFileAfterTheCluster guards the other half of
// the same file. TmpDir is one directory per machine and the write clears the
// path first, so a shared name would have a second cluster's bootstrap delete
// the first cluster's only credentials — on a node that answers no SSH and has
// already closed its channel.
func TestSaveAdminKubeconfigNamesTheFileAfterTheCluster(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	require.Equal(t, filepath.Join(b.TmpDir, "zykov-admin.kubeconfig"), bctx.adminKubeconfigPath)
	require.FileExists(t, bctx.adminKubeconfigPath)

	// The tmp cleaner spares this file by suffix, so the per-cluster name has to
	// keep the suffix or the credentials are swept away with the rest of TmpDir.
	require.True(t, strings.HasSuffix(bctx.adminKubeconfigPath, cache.AdminKubeconfigName))
}

// The file holds cluster-admin credentials, so how it is created matters as much
// as where. Writing into whatever is already at the path would inherit a mode
// somebody else chose and would follow a symlink somebody else planted; both are
// asserted here, because os.WriteFile passes the name check above and fails both
// of these.
func TestSaveAdminKubeconfigWritesAFreshPrivateFile(t *testing.T) {
	t.Run("a wider mode left by an earlier run is not inherited", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = t.TempDir()

		path := filepath.Join(b.TmpDir, "zykov-admin.kubeconfig")
		require.NoError(t, os.WriteFile(path, []byte("stale"), 0o644))

		require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"cluster-admin credentials must not be left at the mode of whatever was there before")
	})

	t.Run("a symlink at the path is replaced, not followed", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = t.TempDir()

		target := filepath.Join(t.TempDir(), "somebody-elses-file")
		require.NoError(t, os.WriteFile(target, []byte("untouched"), 0o600))

		path := filepath.Join(b.TmpDir, "zykov-admin.kubeconfig")
		require.NoError(t, os.Symlink(target, path))

		require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

		pointedAt, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, "untouched", string(pointedAt),
			"the credentials must not be written through a symlink somebody else planted")

		info, err := os.Lstat(path)
		require.NoError(t, err)
		require.Zero(t, info.Mode()&os.ModeSymlink, "the symlink must have been replaced by a regular file")
	})
}

// immutableTestBootstrapper builds the smallest bootstrapper that can render
// the master payload.
func immutableTestBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext) {
	t.Helper()

	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	opts := options.New()
	// The default CandiDir points into a temporary directory the installer image
	// populates at runtime, which a test machine has no reason to have. Left at
	// the default, this renders from an empty directory and the test only passes
	// where a previous dhctl run happened to leave one behind.
	opts.Global.CandiDir = repoCandiDir(t)

	b := &ClusterBootstrapper{Params: &Params{Options: opts}}

	return b, &bootstrapContext{
		metaConfig: immutableTestMetaConfig(t),
		stateCache: stateCache,
	}
}

// repoCandiDir finds the checkout's own candi directory by walking up from the
// test's working directory.
func repoCandiDir(t *testing.T) string {
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

// immutableHandoffTestServer serves the node's side of the bootstrap channel and
// returns the port it landed on. Bound to :0 rather than the protocol's fixed
// port, so a runner that happens to be using that port does not make this test
// disappear while the suite stays green.
func immutableHandoffTestServer(t *testing.T, material *immutable.HandoffMaterial, handler http.HandlerFunc) int {
	t.Helper()

	certificate, err := tls.X509KeyPair([]byte(material.ServerCertPEM), []byte(material.ServerKeyPEM))
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)
	require.NoError(t, server.Listener.Close())
	server.Listener = listener
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	t.Cleanup(server.Close)

	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return address.Port
}

// immutableWaitingBootstrapper is the smallest bootstrapper collectImmutableKubeconfig
// runs against: no SSH provider, so the channel is the master's address directly.
//
// It also restores real retry behaviour for the duration of the test. init_test.go
// collapses every loop in this binary to a single wait-free attempt, which would
// make a test of "does this loop stop early" pass without exercising anything.
// Borrowed and given back rather than set: nothing in this package calls
// t.Parallel, so these run one at a time.
func immutableWaitingBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext, *immutable.HandoffMaterial) {
	t.Helper()

	inTestEnvironment := libretry.InTestEnvironment
	libretry.InTestEnvironment = false
	t.Cleanup(func() { libretry.InTestEnvironment = inTestEnvironment })

	b, bctx := immutableTestBootstrapper(t)
	bctx.masterIP = "127.0.0.1"
	bctx.masterNodeName = "zykov-master-0"

	material, err := immutable.HandoffMaterialFor(t.Context(), bctx.stateCache, bctx.masterNodeName)
	require.NoError(t, err)

	return b, bctx, material
}

// A node that reports Failed has stopped working towards a control plane, so the
// wait ends with the message it gave instead of polling a dead node for the rest
// of its half-hour budget. The test would take that half hour if BreakIf stopped
// matching.
func TestCollectImmutableKubeconfigStopsOnAFailedNode(t *testing.T) {
	b, bctx, material := immutableWaitingBootstrapper(t)

	port := immutableHandoffTestServer(t, material, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"phase":"Failed","message":"pull the kubelet system extension: 404"}`))
	})

	// Bounded so that a loop which stops treating Failed as terminal fails this
	// assertion instead of running until the test binary is killed, which takes
	// every other test in the package down with it as a panic.
	ctx, cancel := context.WithTimeout(t.Context(), 2*immutableAPIWaitInterval)
	defer cancel()

	started := time.Now()
	_, err := b.collectImmutableKubeconfig(ctx, bctx, port)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pull the kubelet system extension: 404",
		"the node's own message is the only thing that says what went wrong")
	require.Less(t, time.Since(started), immutableAPIWaitInterval,
		"a node that reported Failed must end the wait, not start the next attempt")
}

// channelBroken decides whether a failed attempt arms the reopen, and both
// answers cost something when wrong: treating a refused dial as a break rebuilds
// the tunnel every few seconds throughout the half hour a healthy node spends
// installing itself, while missing a real break spends the rest of that half
// hour on a port nothing listens on.
func TestChannelBroken(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		broken bool
	}{
		{name: "a stream that ends mid-answer", err: io.EOF, broken: true},
		{name: "a truncated body", err: io.ErrUnexpectedEOF, broken: true},
		{name: "the peer reset it", err: syscall.ECONNRESET, broken: true},
		{name: "writing to a closed pipe", err: syscall.EPIPE, broken: true},
		{name: "wrapped, the way an HTTP client returns it", err: fmt.Errorf("reach the channel: %w", io.EOF), broken: true},
		{
			name:   "a refused dial, which is also a node still installing itself",
			err:    syscall.ECONNREFUSED,
			broken: false,
		},
		{name: "the node saying it is not ready", err: errors.New("the first master is not ready"), broken: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.broken, channelBroken(tt.err))
		})
	}
}
