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
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

// MetaConfig.CachePath() is built from ClusterPrefix and ProviderName, and Prepare assigns both
// only on the cloud branch. Every static and every managed cluster therefore shares one identity,
// one state cache directory and everything in it: the cluster UUID, the master hosts and the
// tombstone a finished bootstrap leaves behind. primaryCacheIdentity keeps the cloud string
// byte-identical and names the other kinds after what the command was actually pointed at.
//
// Host names only: SSH user, port, bastion and keys are excluded on purpose, so that
// bootstrap-phase abort does not have to repeat them and adding --ssh-bastion-host between a
// failed bootstrap and its abort does not move the cluster to another directory. Sorted and
// de-duplicated, so --ssh-host B --ssh-host A and --ssh-host A --ssh-host B are one cluster.
//
// The result is a valid RFC 1123 subdomain of at most 63 characters on every arm but the
// kubeconfig one, whose length is pre-existing behaviour shared with dhctl converge: with
// --kube-cachestore-namespace and no --kube-cachestore-name the identity is used raw as a
// Kubernetes Secret name and copied into a label value.
func primaryCacheIdentity(m *config.MetaConfig, hosts []sshconfig.Host, kube options.KubeOptions) string {
	if m.ClusterPrefix != "" && m.ProviderName != "" {
		return m.CachePath()
	}

	if names := canonHosts(hosts); len(names) > 0 {
		return "ssh-" + stringsutil.Sha256EncodeWithFirstLettersOfHash(strings.Join(names, ","), 32)
	}

	if kube.Config != "" {
		return cache.GetCacheIdentityFromKubeconfig(kube.Config, kube.ConfigContext)
	}

	if kube.InCluster {
		return "in-cluster"
	}

	// A static bootstrap of the host it runs on carries no address of its own. Refusing it here
	// would break a bootstrap the interactive branch of bootstrapPreparation supports.
	return m.CachePath()
}

func canonHosts(hosts []sshconfig.Host) []string {
	names := make([]string, 0, len(hosts))

	for _, h := range hosts {
		if h.Host != "" {
			names = append(names, h.Host)
		}
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// cacheIdentity resolves the identity the run opens its state cache under, keeping an
// upgrade-era cache findable.
//
// Installations that predate primaryCacheIdentity hold their state under the collapsed
// CachePath() string, and bootstrap-phase abort finds the record of what a half-finished
// bootstrap provisioned by recomputing it. So the first cluster that would otherwise leave that
// directory behind claims it instead and keeps using it for its whole life. Nothing is renamed,
// copied or merged: the directory stays where an older dhctl would look for it too.
func cacheIdentity(
	ctx context.Context,
	m *config.MetaConfig,
	hosts []sshconfig.Host,
	kube options.KubeOptions,
	cacheDir string,
) string {
	legacy := m.CachePath()

	identity := primaryCacheIdentity(m, hosts, kube)
	if identity == legacy {
		return legacy
	}

	// A run that already holds a bootstrap of its own has nothing to adopt: the legacy directory
	// belongs to whatever else shares the cache directory, and resuming from it would leave this
	// cluster's own record behind unread.
	if holdsBootstrap(filepath.Join(cacheDir, stringsutil.Sha256Encode(identity))) {
		return identity
	}

	if claimLegacyCache(ctx, cacheDir, legacy, identity) {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("State cache of identity %q adopted as %q", legacy, identity))
		return legacy
	}

	return identity
}

// claimLegacyCache reports whether identity owns the state cache directory of the collapsed
// legacy identity, claiming it when it is unowned and holds a bootstrap worth resuming.
//
// The claim is a file beside the directory rather than inside it: the state directory is
// round-tripped whole to Commander by ExtractDhctlState, emptied by CleanWithExceptions and
// wiped by a cache init that resets the initial state, and none of those must see or destroy it.
func claimLegacyCache(ctx context.Context, cacheDir, legacy, identity string) bool {
	legacyDir := filepath.Join(cacheDir, stringsutil.Sha256Encode(legacy))
	marker := legacyDir + ".owner"

	if _, err := os.Stat(marker); err == nil {
		return claimedBy(marker, identity)
	}

	// A tombstoned directory is a bootstrap that ran to the end; adopting it would import the
	// refusal. A directory without a uuid holds nothing abort or a resume could use.
	if _, err := os.Stat(filepath.Join(legacyDir, state.TombstoneKey)); err == nil {
		return false
	}

	if !holdsBootstrap(legacyDir) {
		return false
	}

	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return claimedBy(marker, identity)
	}

	_, writeErr := f.WriteString(identity)

	if err := cmp.Or(writeErr, f.Close()); err != nil {
		// A marker holding less than the whole identity matches nobody, and O_EXCL means nobody
		// can rewrite it either: left behind, it makes the directory unadoptable by anyone.
		os.Remove(marker)
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot record the owner of the state cache %s: %v", legacyDir, err))

		return false
	}

	return true
}

// holdsBootstrap reports whether a state cache directory holds the record bootstrap-phase abort
// gates on and a resumed bootstrap reads its cluster UUID from.
func holdsBootstrap(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "uuid"))

	return err == nil
}

func claimedBy(marker, identity string) bool {
	owner, err := os.ReadFile(marker)

	return err == nil && string(owner) == identity
}

func connectionHosts(i *providerinitializer.SSHProviderInitializer) []sshconfig.Host {
	cfg := i.GetConfig()
	if cfg == nil {
		return nil
	}

	return cfg.Hosts
}
