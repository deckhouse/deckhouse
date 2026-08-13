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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// TestBuildImmutableMasterPayloadIsBase64CloudConfig pins the contract: the
// payload travels in the "cloudConfig" tfvar, which every provider's terraform
// base64decodes before writing the cloud-init secret.
func TestBuildImmutableMasterPayloadIsBase64CloudConfig(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)

	payload, err := b.buildImmutableMasterPayload(t.Context(), bctx, "example-master-0")
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

// TestAdminKubeconfigFromCache covers the rerun: it re-enters this step with the
// node's channel closing or shut, and must read the credentials the first attempt
// saved instead of waiting half an hour on a listener that is gone.
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

		saved := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
		require.NoError(t, os.WriteFile(saved, []byte(collected), 0o600))
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, saved))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err)
		require.Equal(t, collected, string(content))
		require.Equal(t, saved, path)
	})

	// The record does not prove the channel is closed, and the operator may have
	// moved the file; refusing here would make the bootstrap unfinishable, with
	// no flag to clear the record.
	t.Run("the recorded file has been moved away", func(t *testing.T) {
		stateCache, err := cache.NewStateCache(t.TempDir())
		require.NoError(t, err)

		missing := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
		require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, missing))

		content, path, err := adminKubeconfigFromCache(t.Context(), stateCache)
		require.NoError(t, err, "an unreadable record must fall through to the node, not end the bootstrap")
		require.Nil(t, content)
		require.Empty(t, path)
	})
}

// Once the handover is confirmed the node closes its channel for good and the
// installer's client key is dropped, so a kubeconfig that has gone missing since
// cannot be collected again. Saying so beats spending the half-hour collection
// budget on a refused port and then failing on the missing key.
func TestAdminKubeconfigFromCacheStopsWhenTheHandoverIsOver(t *testing.T) {
	stateCache, err := cache.NewStateCache(t.TempDir())
	require.NoError(t, err)

	_, err = immutable.HandoffMaterialFor(t.Context(), stateCache, "example-master-0")
	require.NoError(t, err)
	require.NoError(t, immutable.ForgetHandoffClientKey(t.Context(), stateCache))

	missing := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
	require.NoError(t, immutable.SaveCollectedKubeconfig(t.Context(), stateCache, missing))

	_, _, err = adminKubeconfigFromCache(t.Context(), stateCache)
	require.Error(t, err, "there is no second collection to fall through to")
	require.Contains(t, err.Error(), "cannot be collected a second time")
}

// A rerun re-enters this step with the credentials already in hand, which skips
// the branch that writes them — so a --kubeconfig-out named for the first time
// on that rerun would be silently ignored.
func TestSaveAdminKubeconfigOnRerunHonoursKubeconfigOut(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	content := []byte("apiVersion: v1\nkind: Config\n")
	collected := filepath.Join(t.TempDir(), "example-admin.kubeconfig")
	require.NoError(t, os.WriteFile(collected, content, 0o600))

	out := filepath.Join(t.TempDir(), "prod.kubeconfig")
	b.Options.Bootstrap.KubeconfigOut = out

	require.NoError(t, b.saveAdminKubeconfigOnRerun(t.Context(), content, bctx, collected))

	written, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Equal(t, content, written)
	require.Equal(t, out, bctx.adminKubeconfigPath)

	// The record follows the file, or the next rerun reads the path this one left.
	recorded, err := immutable.LoadCollectedKubeconfig(t.Context(), bctx.stateCache)
	require.NoError(t, err)
	require.Equal(t, out, recorded)

	// Nothing to do when the file is already where the flag names it: the write
	// clears the path first, and that file is the only copy of the credentials.
	b.Options.Bootstrap.KubeconfigOut = collected
	require.NoError(t, os.Remove(collected))
	require.NoError(t, b.saveAdminKubeconfigOnRerun(t.Context(), content, bctx, collected))
	require.NoFileExists(t, collected, "the file that is already in place must not be rewritten")
}

// The record has to be written before ConfirmCollected shuts the node's channel
// for good; saveAdminKubeconfig is the last point where that ordering holds, so a
// rerun that died anywhere after it finds the file rather than a dead channel.
func TestSaveAdminKubeconfigRecordsThePathBeforeTheChannelCloses(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	recorded, err := immutable.LoadCollectedKubeconfig(t.Context(), bctx.stateCache)
	require.NoError(t, err)
	require.Equal(t, bctx.adminKubeconfigPath, recorded,
		"the rerun path must be usable from the moment the file exists, not from the confirmation")
}

// TestSaveAdminKubeconfigNamesTheFileAfterTheCluster: TmpDir is one directory per
// machine and the write clears the path first, so a shared name would have a second
// cluster's bootstrap delete the first cluster's only credentials.
func TestSaveAdminKubeconfigNamesTheFileAfterTheCluster(t *testing.T) {
	b, bctx := immutableTestBootstrapper(t)
	b.TmpDir = t.TempDir()

	require.NoError(t, b.saveAdminKubeconfig(t.Context(), []byte("apiVersion: v1\nkind: Config\n"), bctx))

	require.Equal(t, filepath.Join(b.TmpDir, "example-admin.kubeconfig"), bctx.adminKubeconfigPath)
	require.FileExists(t, bctx.adminKubeconfigPath)

	// The tmp cleaner spares this file by suffix, so the per-cluster name has to
	// keep the suffix or the credentials are swept away with the rest of TmpDir.
	require.True(t, strings.HasSuffix(bctx.adminKubeconfigPath, cache.AdminKubeconfigName))
}

// The file holds cluster-admin credentials: writing into whatever is already at the
// path would inherit a foreign mode and follow a planted symlink. Both are asserted
// because os.WriteFile passes the name check above and fails both of these.
func TestSaveAdminKubeconfigWritesAFreshPrivateFile(t *testing.T) {
	t.Run("a wider mode left by an earlier run is not inherited", func(t *testing.T) {
		b, bctx := immutableTestBootstrapper(t)
		b.TmpDir = t.TempDir()

		path := filepath.Join(b.TmpDir, "example-admin.kubeconfig")
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

		path := filepath.Join(b.TmpDir, "example-admin.kubeconfig")
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
	// The default CandiDir points into a directory only the installer image
	// populates; left as is, the test would depend on leftovers of a previous
	// dhctl run.
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
				"nodeletSysext":          digest,
			},
			"nodeManager": {"olcedar": digest},
			"common":      {"pause": digest},
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
// port, so a busy port cannot silently skip this test.
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
// runs against: no SSH provider. It also restores real retry behaviour (init_test.go
// collapses every loop to one attempt); safe to swap globally — no t.Parallel here.
func immutableWaitingBootstrapper(t *testing.T) (*ClusterBootstrapper, *bootstrapContext, *immutable.HandoffMaterial) {
	t.Helper()

	inTestEnvironment := libretry.InTestEnvironment
	libretry.InTestEnvironment = false
	t.Cleanup(func() { libretry.InTestEnvironment = inTestEnvironment })

	b, bctx := immutableTestBootstrapper(t)
	bctx.masterIP = "127.0.0.1"
	bctx.masterNodeName = "example-master-0"

	material, err := immutable.HandoffMaterialFor(t.Context(), bctx.stateCache, bctx.masterNodeName)
	require.NoError(t, err)

	return b, bctx, material
}

// A node that reports Failed has stopped working towards a control plane, so the
// wait must end with its message instead of polling for the rest of the half-hour
// budget. The test would take that half hour if BreakIf stopped matching.
func TestCollectImmutableKubeconfigStopsOnAFailedNode(t *testing.T) {
	b, bctx, material := immutableWaitingBootstrapper(t)

	port := immutableHandoffTestServer(t, material, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"phase":"Failed","message":"pull the kubelet system extension: 404"}`))
	})

	// Bounded so that a loop which stops treating Failed as terminal fails this
	// assertion instead of running until the test binary is killed, which takes
	// every other test in the package down with it as a panic.
	ctx, cancel := context.WithTimeout(t.Context(), 2*waitAPIServerUp.interval)
	defer cancel()

	started := time.Now()
	_, err := b.collectImmutableKubeconfig(ctx, bctx, port)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pull the kubelet system extension: 404",
		"the node's own message is the only thing that says what went wrong")
	require.Less(t, time.Since(started), waitAPIServerUp.interval,
		"a node that reported Failed must end the wait, not start the next attempt")
}

// The bootstrap token of a NodeGroup is minted by an asynchronous node-manager
// hook, so the first read of a young master group finds none. Without a retry
// the whole multi-master bootstrap ends there — after the first master is up.
func TestBuildImmutableJoinPayloadWaitsForTheBootstrapToken(t *testing.T) {
	const token = "abcdef.0123456789abcdef"

	// init_test.go collapses every loop to one attempt; safe to swap globally —
	// no t.Parallel here.
	inTestEnvironment := libretry.InTestEnvironment
	libretry.InTestEnvironment = false
	t.Cleanup(func() { libretry.InTestEnvironment = inTestEnvironment })

	kubeCl := client.NewFakeKubernetesClient()
	createJoinInputsWithoutToken(t, kubeCl)

	// Published after the first attempt has already failed, the way the hook
	// publishes it. Bounded by the context so a lost retry fails the test in
	// seconds instead of sitting out the whole budget.
	go func() {
		time.Sleep(immutableJoinTokenDelay)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-token-abcdef",
				Namespace: global.ConfigsNS,
				Labels:    map[string]string{bootstrapTokenNGLabel: global.MasterNodeGroupName},
			},
			Type: corev1.SecretTypeBootstrapToken,
			Data: map[string][]byte{"token-id": []byte("abcdef"), "token-secret": []byte("0123456789abcdef")},
		}
		_, _ = kubeCl.CoreV1().Secrets(global.ConfigsNS).Create(context.Background(), secret, metav1.CreateOptions{})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	payload, err := buildImmutableJoinPayload(ctx, kubeCl, immutableTestMetaConfig(t), "example-master-1")
	require.NoError(t, err, "a token that is not published yet is what the wait exists for")

	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	require.Contains(t, string(document), token, "the payload must carry the token the cluster published")
}

// immutableJoinTokenDelay makes the token appear while the first attempt is
// already over: shorter than the loop's interval, so the second attempt finds it.
const immutableJoinTokenDelay = 100 * time.Millisecond

// createJoinInputsWithoutToken publishes everything a joining master reads from
// the cluster except the bootstrap token.
func createJoinInputsWithoutToken(t *testing.T, kubeCl *client.KubernetesClient) {
	t.Helper()

	_, err := kubeCl.CoreV1().ConfigMaps(global.ConfigsNS).Create(t.Context(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: clusterCAConfigMap, Namespace: global.ConfigsNS},
		Data:       map[string]string{clusterCAKey: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	port := int32(immutable.APIServerPort)
	portName := apiServerPortName
	_, err = kubeCl.DiscoveryV1().EndpointSlices(apiServerEndpointSliceNS).Create(t.Context(), &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: apiServerEndpointSliceName, Namespace: apiServerEndpointSliceNS},
		Endpoints:  []discoveryv1.Endpoint{{Addresses: []string{"192.168.1.10"}}},
		Ports:      []discoveryv1.EndpointPort{{Name: &portName, Port: &port}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}
