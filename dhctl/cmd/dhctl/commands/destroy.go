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
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
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
	// Without kube flags a cluster whose nodes run no SSH server was impossible
	// to delete: the infrastructure state lives in the cluster, and reaching it
	// needs a kubeconfig, not a shell on a node.
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

		cacheIdentity, sshProvider, err := destroyCacheIdentity(ctx, opts, sshProviderInitializer)
		if err != nil {
			return err
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

// destroyCacheIdentity names the cluster whose infrastructure state destroy will
// delete, and the SSH provider it reaches it through when there is one. A wrong
// identity silently addresses somebody else's state.
func destroyCacheIdentity(
	ctx context.Context,
	opts *options.Options,
	sshProviderInitializer *providerinitializer.SSHProviderInitializer,
) (string, libcon.SSHProvider, error) {
	// A kubeconfig plus an SSH host is refused rather than ranked: --kubeconfig
	// also reads DHCTL_CLI_KUBE_CONFIG.
	sshHostConfigured := sshProviderInitializer != nil && sshProviderInitializer.CheckHosts(ctx)
	kubeSourceConfigured := opts.Kube.Config != "" || opts.Kube.InCluster
	if sshHostConfigured && kubeSourceConfigured {
		// Tested before either source is read, so the operator hears about the
		// collision rather than about a kubeconfig they never meant to name.
		source := "--kube-client-from-cluster"
		if opts.Kube.Config != "" {
			source = "--kubeconfig " + opts.Kube.Config
		}
		return "", nil, fmt.Errorf(
			"%s and an SSH host both name a cluster, and they may be different clusters: "+
				"the Kubernetes source is where the infrastructure state is read from, the SSH host is where it is not. "+
				"Note that --kubeconfig also reads the DHCTL_CLI_KUBE_CONFIG environment variable. "+
				"Unset one of them and run destroy again",
			source,
		)
	}

	var sshProvider libcon.SSHProvider
	cacheIdentity := ""
	if opts.Kube.Config != "" {
		// NOT GetCacheIdentityFromKubeconfig: that hashes the path, and every
		// immutable bootstrap writes to the same path — all clusters on this
		// machine would then share the cache destroy reads its state from.
		identity, err := kubeconfigClusterIdentity(opts.Kube.Config, opts.Kube.ConfigContext)
		if err != nil {
			return "", nil, fmt.Errorf("identify the cluster from %s: %w", opts.Kube.Config, err)
		}
		cacheIdentity = identity
	}
	if opts.Kube.InCluster {
		// No conflict with --kubeconfig to handle here: lib-connection's
		// Config.IsConflict rejects two set modes while the providers are built by
		// the caller, and that error is returned at once.
		identity, err := inClusterCacheIdentity()
		if err != nil {
			return "", nil, err
		}
		cacheIdentity = identity
	}
	if sshHostConfigured {
		provider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return "", nil, err
		}
		sshClient, err := provider.Client(ctx)
		if err != nil {
			return "", nil, err
		}
		sshProvider = provider
		cacheIdentity = sshClient.Check().String()
	}
	if cacheIdentity == "" {
		return "", nil, errors.New("nothing identifies this cluster: pass --ssh-host for a cluster with SSH, or --kubeconfig for one without")
	}

	return cacheIdentity, sshProvider, nil
}

// inClusterCacheIdentity names the cluster from the API server this pod talks
// to, rather than from a bare "in-cluster" constant that every in-cluster run
// on this machine would share — the same collision one directory further up.
func inClusterCacheIdentity() (string, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" {
		return "", errors.New(
			"nothing identifies this cluster: --kube-client-from-cluster is set but KUBERNETES_SERVICE_HOST is empty, so dhctl runs outside a cluster",
		)
	}
	return "in-cluster-" + net.JoinHostPort(host, port), nil
}

// kubeconfigClusterIdentity names the cluster a kubeconfig points at by what is
// inside it. The CA is required: the printed bastion-forward line retargets every
// immutable cluster to https://127.0.0.1:6445, so the address alone identifies nothing.
func kubeconfigClusterIdentity(path, contextName string) (string, error) {
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return "", err
	}
	// LoadFromFile leaves certificate-authority as written, so a relative one is
	// relative to the kubeconfig's own directory rather than to the cwd.
	if err := clientcmd.ResolveLocalPaths(cfg); err != nil {
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

	// certificate-authority is a path, so an unread file would leave the address
	// hashed on its own — the very collision this function exists to prevent,
	// and a silent one.
	certificateAuthority := cluster.CertificateAuthorityData
	if len(certificateAuthority) == 0 && cluster.CertificateAuthority != "" {
		certificateAuthority, err = os.ReadFile(cluster.CertificateAuthority)
		if err != nil {
			return "", fmt.Errorf("read the cluster CA %s: %w", cluster.CertificateAuthority, err)
		}
	}
	if len(certificateAuthority) == 0 {
		return "", fmt.Errorf(
			"context %q carries no cluster CA (certificate-authority or certificate-authority-data): "+
				"its server address alone does not tell this cluster from another one reached at the same address, "+
				"and destroy reads the infrastructure state out of a cache keyed by that answer",
			contextName,
		)
	}

	// The CA alone, deliberately not the server address: a destroy resumed through
	// the bastion forward carries a rewritten address, so hashing it would hand the
	// resume an empty cache with no infrastructure state to finish from.
	return fmt.Sprintf("kubeconfig-%x", sha256.Sum256(certificateAuthority)), nil
}
