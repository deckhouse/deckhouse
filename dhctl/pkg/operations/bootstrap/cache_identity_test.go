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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

const (
	// The identity every static and every managed cluster collapsed to, and the one existing
	// installations hold their state under. Spelled out rather than computed from CachePath(), so
	// that a change to the formula fails here instead of moving both sides of the comparison.
	legacyIdentity = "--terraform-state-cache"
	legacyDirName  = "3a04ff27424225d5cb94565816be0989785d21017e93b098c81974947d36c334"

	// sha256("10.0.0.1"), first 32 hex characters.
	oneHostIdentity = "ssh-f5047344122f0dee9974ba6761e61c6b"
	// sha256("a,b"), first 32 hex characters.
	twoHostsIdentity = "ssh-1eb7c54d52831bbfe8942af0b1c56b74"
	// sha256("a"), first 32 hex characters.
	hostAIdentity = "ssh-ca978112ca1bbdcafac231b39a23dc4d"
)

func hosts(names ...string) []sshconfig.Host {
	out := make([]sshconfig.Host, 0, len(names))
	for _, n := range names {
		out = append(out, sshconfig.Host{Host: n})
	}

	return out
}

func staticConfig() *config.MetaConfig {
	return &config.MetaConfig{ClusterType: config.StaticClusterType}
}

func TestPrimaryCacheIdentity(t *testing.T) {
	cloud := &config.MetaConfig{ClusterType: config.CloudClusterType, ClusterPrefix: "dev", ProviderName: "yandex"}

	cases := []struct {
		name     string
		meta     *config.MetaConfig
		hosts    []sshconfig.Host
		kube     options.KubeOptions
		expected string
	}{
		{
			name:     "a cloud cluster keeps the identity every existing installation holds its state under",
			meta:     cloud,
			expected: "dev-yandex-terraform-state-cache",
		},
		{
			name:     "no flag combination moves a cloud cluster",
			meta:     cloud,
			hosts:    hosts("10.0.0.1"),
			kube:     options.KubeOptions{Config: "/x/kubeconfig", InCluster: true},
			expected: "dev-yandex-terraform-state-cache",
		},
		{
			name:     "a static cluster is named after the hosts it was pointed at",
			meta:     staticConfig(),
			hosts:    hosts("10.0.0.1"),
			expected: oneHostIdentity,
		},
		{
			name:     "the order of --ssh-host does not name a different cluster",
			meta:     staticConfig(),
			hosts:    hosts("b", "a"),
			expected: twoHostsIdentity,
		},
		{
			name:     "a repeated or empty host does not name a different cluster",
			meta:     staticConfig(),
			hosts:    hosts("a", "a", ""),
			expected: hostAIdentity,
		},
		{
			// bootstrap-phase abort defines no kube flags, so a cluster bootstrapped with both
			// would be unabortable if kubeconfig won here.
			name:     "hosts beat a kubeconfig",
			meta:     staticConfig(),
			hosts:    hosts("10.0.0.1"),
			kube:     options.KubeOptions{Config: "/x/kubeconfig"},
			expected: oneHostIdentity,
		},
		{
			name:     "a managed cluster is named after the kubeconfig it is reached through",
			meta:     &config.MetaConfig{},
			kube:     options.KubeOptions{Config: "/x/kubeconfig"},
			expected: "kubeconfig-437d9e1d2d736e6c33a80572f2c257aa77190446851b7bfa4f3825349895a461",
		},
		{
			name:     "the kubeconfig context is part of the identity",
			meta:     &config.MetaConfig{},
			kube:     options.KubeOptions{Config: "/x/kubeconfig", ConfigContext: "prod"},
			expected: "kubeconfig-7b9c585da85d49549f59f94c0a51c236c68c7df44a853d1f5a841e77879a4678",
		},
		{
			name:     "a managed cluster reached from inside itself",
			meta:     &config.MetaConfig{},
			kube:     options.KubeOptions{InCluster: true},
			expected: "in-cluster",
		},
		{
			// A static bootstrap of the current host is legal and carries no address; refusing it
			// here would break it.
			name:     "nothing to name the cluster after falls through to the legacy identity",
			meta:     staticConfig(),
			expected: legacyIdentity,
		},
		{
			name:     "a nil host slice is not a panic",
			meta:     staticConfig(),
			hosts:    nil,
			expected: legacyIdentity,
		},
	}

	// With --kube-cachestore-namespace and no --kube-cachestore-name the identity is used raw as
	// the name of a Kubernetes Secret and copied into a label value, capped at 63 characters. The
	// kubeconfig arm is exempt: its length is pre-existing behaviour shared with dhctl converge.
	dns1123 := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := primaryCacheIdentity(tc.meta, tc.hosts, tc.kube)

			require.Equal(t, tc.expected, identity)

			// The legacy fallthrough and the kubeconfig arm are both shipped strings - the first
			// is what CachePath() has always produced, the second is what dhctl converge already
			// writes - so neither is this change's to fix.
			if identity == legacyIdentity || strings.HasPrefix(identity, "kubeconfig-") {
				return
			}

			require.Regexp(t, dns1123, identity, "the identity names a Kubernetes Secret on the kube cache store")
			require.LessOrEqual(t, len(identity), 63, "a Kubernetes label value is capped at 63 characters")
		})
	}
}

// TestCacheIdentityAdoptsTheLegacyCache simulates the upgrade the compatibility requirement is
// about: a cache written by an older dhctl under the collapsed identity, then read by this code.
//
// It never calls cache.Init. The package-level sync.Once in pkg/state/cache makes a second Init a
// silent no-op that returns a nil error, so an upgrade test built out of two global inits would
// pass without proving anything.
func TestCacheIdentityAdoptsTheLegacyCache(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, legacyDirName)

	require.Equal(t, legacyDirName, stringsutil.Sha256Encode(legacyIdentity),
		"existing installations hold their state in this directory",
	)

	clusterUUID := "931fe431-5446-440c-a5b8-91a607d1ea4e"

	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "uuid"), []byte(clusterUUID), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "base-infrastructure.tfstate"), []byte("tfstate"), 0o600))

	legacyCache, err := utilcache.NewStateCache(legacyDir)
	require.NoError(t, err)
	require.NoError(t, state.SaveMasterHosts(ctx, legacyCache, map[string]string{"master-0": "10.0.0.1"}))

	before, err := os.ReadDir(legacyDir)
	require.NoError(t, err)

	identity := cacheIdentity(ctx, staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir)

	require.Equal(t, legacyIdentity, identity,
		"bootstrap-phase abort finds the record of a half-finished bootstrap here and refuses without it",
	)

	owner, err := os.ReadFile(legacyDir + ".owner")
	require.NoError(t, err)
	require.Equal(t, oneHostIdentity, string(owner))

	after, err := os.ReadDir(legacyDir)
	require.NoError(t, err)
	require.Equal(t, names(before), names(after), "the legacy directory is adopted where it is, never moved")

	adopted, err := utilcache.NewStateCache(filepath.Join(dir, stringsutil.Sha256Encode(identity)))
	require.NoError(t, err)

	loaded, err := adopted.Load(ctx, "uuid")
	require.NoError(t, err)
	require.Equal(t, clusterUUID, string(loaded), "the adopted directory has to be a usable cache, not just a pile of files")

	require.Equal(t, legacyIdentity, cacheIdentity(ctx, staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir),
		"the claim has to survive into the next run of the same cluster",
	)
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}

	return out
}

func TestCacheIdentityRefusesToAdopt(t *testing.T) {
	withLegacyDir := func(t *testing.T, files map[string]string) string {
		t.Helper()

		dir := t.TempDir()
		legacyDir := filepath.Join(dir, legacyDirName)
		require.NoError(t, os.MkdirAll(legacyDir, 0o755))

		for name, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(legacyDir, name), []byte(content), 0o600))
		}

		return dir
	}

	requireNoMarker := func(t *testing.T, dir string) {
		t.Helper()

		_, err := os.Stat(filepath.Join(dir, legacyDirName+".owner"))
		require.True(t, os.IsNotExist(err), "an unadopted directory must not be claimed")
	}

	for name, files := range map[string]map[string]string{
		"a tombstoned directory holding state":                      {"uuid": "u", state.TombstoneKey: ""},
		"a directory a finished bootstrap reduced to its tombstone": {state.TombstoneKey: ""},
		"a directory with no uuid":                                  {"base-infrastructure.tfstate": "tfstate"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := withLegacyDir(t, files)

			require.Equal(t, oneHostIdentity, cacheIdentity(t.Context(), staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir))
			requireNoMarker(t, dir)
		})
	}

	// Not built by withLegacyDir: the legacy directory is absent here, not empty, and the two
	// answers come from different arms.
	t.Run("no legacy directory at all", func(t *testing.T) {
		dir := t.TempDir()

		require.Equal(t, oneHostIdentity, cacheIdentity(t.Context(), staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir))
		requireNoMarker(t, dir)
	})

	t.Run("a directory another cluster already claimed", func(t *testing.T) {
		dir := withLegacyDir(t, map[string]string{"uuid": "u"})
		marker := filepath.Join(dir, legacyDirName+".owner")
		require.NoError(t, os.WriteFile(marker, []byte(hostAIdentity), 0o600))

		require.Equal(t, oneHostIdentity, cacheIdentity(t.Context(), staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir))

		owner, err := os.ReadFile(marker)
		require.NoError(t, err)
		require.Equal(t, hostAIdentity, string(owner), "the first claimant keeps the directory")
	})

	t.Run("a legacy directory when the run already holds a bootstrap of its own", func(t *testing.T) {
		dir := withLegacyDir(t, map[string]string{"uuid": "the-stranger-in-the-shared-directory"})

		ownDir := filepath.Join(dir, stringsutil.Sha256Encode(oneHostIdentity))
		require.NoError(t, os.MkdirAll(ownDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(ownDir, "uuid"), []byte("the-cluster-being-bootstrapped"), 0o600))

		identity := cacheIdentity(t.Context(), staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir)

		require.Equal(t, oneHostIdentity, identity)
		requireNoMarker(t, dir)

		resumed, err := os.ReadFile(filepath.Join(dir, stringsutil.Sha256Encode(identity), "uuid"))
		require.NoError(t, err)
		require.Equal(t, "the-cluster-being-bootstrapped", string(resumed),
			"a run must resume its own bootstrap, not the one the shared directory happens to hold",
		)
	})

	t.Run("a cloud cluster never looks at it", func(t *testing.T) {
		dir := withLegacyDir(t, map[string]string{"uuid": "u"})
		cloud := &config.MetaConfig{ClusterType: config.CloudClusterType, ClusterPrefix: "dev", ProviderName: "yandex"}

		require.Equal(t, "dev-yandex-terraform-state-cache", cacheIdentity(t.Context(), cloud, hosts("10.0.0.1"), options.KubeOptions{}, dir))
		requireNoMarker(t, dir)
	})

	t.Run("a cache directory that does not exist", func(t *testing.T) {
		cloud := &config.MetaConfig{ClusterType: config.CloudClusterType, ClusterPrefix: "dev", ProviderName: "yandex"}

		require.Equal(t, "dev-yandex-terraform-state-cache",
			cacheIdentity(t.Context(), cloud, nil, options.KubeOptions{}, filepath.Join(t.TempDir(), "absent")),
		)
	})
}

// TestCacheIdentitySeparatesStaticClusters is the precondition behind the SIGSEGV the identity
// change exists to remove: a completed bootstrap leaves its directory holding nothing but a
// tombstone, and the next bootstrap on the same machine used to open that very directory, fail,
// and hand a nil *StateCache back through a non-nil interface.
func TestCacheIdentitySeparatesStaticClusters(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	first := cacheIdentity(ctx, staticConfig(), hosts("10.0.0.1"), options.KubeOptions{}, dir)
	second := cacheIdentity(ctx, staticConfig(), hosts("10.0.0.2"), options.KubeOptions{}, dir)

	require.NotEqual(t, first, second, "two static clusters must not share one state cache")

	firstDir := filepath.Join(dir, stringsutil.Sha256Encode(first))
	require.NoError(t, os.MkdirAll(firstDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(firstDir, state.TombstoneKey), nil, 0o600))

	_, err := utilcache.NewStateCache(firstDir)
	require.Error(t, err, "the first cluster's directory is exhausted")

	_, err = utilcache.NewStateCache(filepath.Join(dir, stringsutil.Sha256Encode(second)))
	require.NoError(t, err, "and the second cluster is not refused by it")
}
