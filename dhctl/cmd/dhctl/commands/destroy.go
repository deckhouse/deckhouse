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

package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/tools/clientcmd"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	tmp "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	destroyApprovalsMessage = `You will be asked for approval multiple times.
If you understand what you are doing, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
	destroyCacheErrorMessage = `Create cache:
	Error: %v

	The Kubernetes cluster was probably already deleted.
	If you want to continue, please delete the cache folder manually.
`
)

func DefineDestroyCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, config.NewConnectionConfigParser(opts))
	// Destroy asked for SSH and nothing else, which made a cluster whose nodes
	// run no SSH server impossible to delete: created but not removable. The
	// infrastructure state lives in the cluster, and reaching it needs a
	// kubeconfig, not a shell on a node.
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineCacheFlags(cmd, &opts.Cache)
	app.DefineSanityFlags(cmd, &opts.Global)
	app.DefineDestroyResourcesFlags(cmd, &opts.Destroy)
	app.DefineTFResourceManagementTimeout(cmd, &opts.Cache)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		l := logger.FromContext(ctx)

		span := telemetry.SpanFromContext(ctx)
		span.SetAttributes(opts.ToSpanAttributes()...)

		params := app.ProviderParams(&opts.Global, logger.FromContext(ctx))

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(ctx, params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithKubeConfig(opts.Kube.Config, opts.Kube.ConfigContext, opts.Kube.InCluster),
		)
		if err != nil {
			// No SSH hosts is not a failure any more: it is what an immutable
			// cluster looks like. Converge already tolerates it the same way.
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		// The cache is keyed by whatever identifies this cluster. With SSH that is
		// the connection string; without it, the kubeconfig. Getting this wrong
		// does not fail loudly — it silently addresses somebody else's state.
		var sshProvider libcon.SSHProvider
		cacheIdentity := ""
		if opts.Kube.InCluster {
			cacheIdentity = "in-cluster"
		}
		if sshProviderInitializer != nil && sshProviderInitializer.CheckHosts(ctx) {
			provider, err := sshProviderInitializer.GetSSHProvider(ctx)
			if err != nil {
				return err
			}
			sshClient, err := provider.Client(ctx)
			if err != nil {
				return err
			}
			sshProvider = provider
			cacheIdentity = sshClient.Check().String()
		}
		if opts.Kube.Config != "" {
			// NOT GetCacheIdentityFromKubeconfig: that hashes the path, and on an
			// immutable cluster the path is always the same one the bootstrap wrote
			// its admin kubeconfig to. Every cluster on this machine would then
			// share a cache directory — and destroy reads the terraform state out
			// of that cache, so it would happily tear down somebody else's
			// infrastructure without saying a word. Key by what the file says the
			// cluster IS.
			identity, err := kubeconfigClusterIdentity(opts.Kube.Config, opts.Kube.ConfigContext)
			if err != nil {
				return fmt.Errorf("identify the cluster from %s: %w", opts.Kube.Config, err)
			}
			cacheIdentity = identity
		}
		if cacheIdentity == "" {
			return fmt.Errorf("nothing identifies this cluster: pass --ssh-host for a cluster with SSH, or --kubeconfig for one without")
		}

		if err = cache.Init(ctx, cacheIdentity, opts.Cache); err != nil {
			return fmt.Errorf(destroyCacheErrorMessage, err)
		}

		destroyerParams := &destroy.Params{
			SSHProvider:   sshProvider,
			KubeProvider:  kubeProvider,
			StateCache:    cache.Global(),
			SkipResources: opts.Destroy.SkipResources,
			Logger:        logger.FromContext(ctx),
			IsDebug:       opts.Global.IsDebug,
			TmpDir:        opts.Global.TmpDir,
			Options:       opts,
		}
		interactive := input.IsTerminal() && !opts.Global.ShowProgress
		if interactive {
			progressCh, finishProgress := phases.InitProgress(ctx, logger.FromContext(ctx), "Destroy cluster")
			defer finishProgress()

			onUpdateFunc := func(progress phases.Progress) error {
				progressCh <- progress
				return nil
			}
			destroyerParams.OnProgressFunc = onUpdateFunc

			// TODO: add sync to make sure progress UI is started
			time.Sleep(100 * time.Millisecond)
		}

		if !opts.Global.SanityCheck {
			l.Warn(fmt.Sprint(destroyApprovalsMessage))

			if !input.NewConfirmation().WithYesByDefault().WithMessage("Do you really want to DELETE all cluster resources?").Ask() {
				return fmt.Errorf("Cluster resource cleanup was not approved")
			}
		}

		destroyer, err := destroy.NewClusterDestroyer(ctx, destroyerParams)
		if err != nil {
			return err
		}

		err = destroyer.DestroyCluster(ctx, opts.Global.SanityCheck)
		if err != nil {
			msg := fmt.Sprintf("Failed to destroy cluster: %v", err)
			tmp.GetGlobalTmpCleaner().DisableCleanup(msg)

			return err
		}

		return nil
	})
}

// kubeconfigClusterIdentity names the cluster a kubeconfig points at, from what
// is inside it rather than from where it sits: the API server address and the
// cluster CA. Two clusters cannot share both.
func kubeconfigClusterIdentity(path, contextName string) (string, error) {
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return "", err
	}
	if contextName == "" {
		contextName = cfg.CurrentContext
	}
	kubeContext, found := cfg.Contexts[contextName]
	if !found {
		return "", fmt.Errorf("no context %q", contextName)
	}
	cluster, found := cfg.Clusters[kubeContext.Cluster]
	if !found {
		return "", fmt.Errorf("context %q names an unknown cluster %q", contextName, kubeContext.Cluster)
	}
	if cluster.Server == "" {
		return "", fmt.Errorf("context %q has no server address", contextName)
	}

	h := sha256.New()
	h.Write([]byte(cluster.Server))
	h.Write(cluster.CertificateAuthorityData)
	return "kubeconfig-" + hex.EncodeToString(h.Sum(nil)), nil
}
