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
	"fmt"
	"os"
	"path/filepath"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

// writeImmutableKubeconfig stores the collected admin kubeconfig in a file the
// Kubernetes client can be built from, with its server URL pointed at the
// address dhctl reaches the API on.
func (b *ClusterBootstrapper) writeImmutableKubeconfig(ctx context.Context, dir string, content []byte) (string, error) {
	// os.CreateTemp reserves a name nothing else holds, at mode 0600; it holds
	// admin credentials, so it is removed again once dhctl exits.
	file, err := os.CreateTemp(dir, "dhctl-immutable-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("create a temporary kubeconfig %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", fmt.Errorf("write the temporary kubeconfig %s: %w", path, err)
	}

	// The happy path removes the file as soon as the client is built; this
	// covers the runs that never get that far.
	tomb.RegisterOnShutdown("Delete the temporary installer kubeconfig", func() {
		removeImmutableKubeconfig(ctx, path)
	})

	return path, nil
}

// saveAdminKubeconfig hands the admin kubeconfig to the operator. Written by
// default: the node runs no sshd and the handoff endpoint has already served
// its one read, so "do not keep it" means a cluster nobody can reach.
func (b *ClusterBootstrapper) saveAdminKubeconfig(ctx context.Context, content []byte, bctx *bootstrapContext) error {
	path := b.Options.Bootstrap.KubeconfigOut
	if path != "" {
		// The same guard the preflight applies, repeated because preflights can be
		// skipped and this one protects the only way into the cluster.
		if err := immutable.CheckKubeconfigOutSurvivesCleanup(ctx, path, b.TmpDir); err != nil {
			return err
		}
	}
	if path == "" {
		// Only the CLI gets a default path. In server mode TmpDir is shared by the
		// whole process, so the default would funnel every cluster's cluster-admin
		// credentials into one file, each run overwriting the last.
		if b.CommanderMode {
			return immutable.ErrKubeconfigOutRequired
		}
		// Named after the cluster: the write below clears the path first, so a
		// second immutable cluster from the same machine would delete the first
		// one's only credentials. The suffix keeps the tmp cleaner off it.
		name := cache.AdminKubeconfigName
		if bctx.metaConfig != nil && bctx.metaConfig.ClusterPrefix != "" {
			name = bctx.metaConfig.ClusterPrefix + "-" + name
		}
		path = filepath.Join(b.TmpDir, name)
	}

	// Removed rather than truncated, then created with O_EXCL: writing into
	// whatever is at that path inherits its mode and follows a symlink someone
	// else put there. O_EXCL is safe because the path was just cleared.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace the admin kubeconfig %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create the admin kubeconfig %s: %w", path, err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("write the admin kubeconfig to %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write the admin kubeconfig to %s: %w", path, err)
	}
	bctx.immutable.kubeconfigPath = path

	// Recorded before the confirmation, not after: a run that died between the two
	// with the record unwritten would leave every rerun dialling a closed channel
	// while the file sat here unread. Returned, not warned, for the same reason.
	if err := immutable.SaveCollectedKubeconfig(ctx, bctx.stateCache, path); err != nil {
		return err
	}

	b.printHowToReachTheCluster(ctx, path, bctx)
	return nil
}

func removeImmutableKubeconfig(ctx context.Context, path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("remove %s: %v", path, err))
	}
}

// newKubeconfigKubeProvider builds a Kubernetes provider that talks to the API
// server directly through the given kubeconfig, with no SSH runner behind it.
func newKubeconfigKubeProvider(ctx context.Context, b *ClusterBootstrapper, kubeconfigPath string) (libcon.KubeProvider, error) {
	kubeConfig := &kube.Config{KubeConfig: kubeconfigPath}

	// A kubeconfig-backed client needs no SSH provider, hence the nil.
	runner, err := provider.GetRunnerInterface(ctx, kubeConfig, b.SSHProviderInitializer.GetSettings(), nil)
	if err != nil {
		return nil, fmt.Errorf("build the Kubernetes runner interface: %w", err)
	}

	// The long wait is over by now: the node has reported Ready and served its
	// kubeconfig, so what these cover is a restarting static pod or a rebuilt
	// forward — a couple of minutes, not the half hour the collection had.
	initParams := libretry.NewEmptyParams(
		libretry.WithWait(waitNodeRegistered.interval),
		libretry.WithAttempts(waitNodeRegistered.attempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)
	readyParams := libretry.NewEmptyParams(
		libretry.WithWait(waitAPIServerReady.interval),
		libretry.WithAttempts(waitAPIServerReady.attempts),
		libretry.WithLogger(dhlog.FromContext(ctx)),
	)

	return provider.NewDefaultKubeProvider(b.SSHProviderInitializer.GetSettings(), kubeConfig, runner).
		WithLoopsParams(provider.KubeProviderLoopsParams{
			InitClient:   initParams,
			WaitingReady: readyParams,
		}), nil
}
