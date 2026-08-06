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
	"net"
	"os"
	"path/filepath"
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

		// The cache is keyed by whatever identifies this cluster. Getting it wrong
		// does not fail loudly — it silently addresses somebody else's state, and
		// destroy reads the terraform state out of that cache and deletes what it
		// finds there.
		//
		// A kubeconfig and an SSH host together are refused rather than ranked.
		// kube.Config.getModes() (lib-connection pkg/kube/config.go) makes
		// OverSSH() false the moment KubeConfig is set, so the client that reads
		// the state talks to the kubeconfig's server and not to anything behind
		// SSH — but --kubeconfig also reads DHCTL_CLI_KUBE_CONFIG, and kingpin
		// cannot tell an exported variable from a typed flag. An operator with
		// that variable set for one cluster who types --ssh-host for another would
		// get the second cluster's address on the prompt and the first cluster's
		// infrastructure deleted. Neither source can be trusted over the other, so
		// this declines to guess.
		var sshProvider libcon.SSHProvider
		sshHostConfigured := sshProviderInitializer != nil && sshProviderInitializer.CheckHosts(ctx)
		if sshHostConfigured && (opts.Kube.Config != "" || opts.Kube.InCluster) {
			// Tested before either source is read, so the operator hears about the
			// collision rather than about a kubeconfig they never meant to name.
			source := "--kube-client-from-cluster"
			if opts.Kube.Config != "" {
				source = "--kubeconfig " + opts.Kube.Config
			}
			return fmt.Errorf(
				"%s and an SSH host both name a cluster, and they may be different clusters: "+
					"the Kubernetes source is where the infrastructure state is read from, the SSH host is where it is not. "+
					"Note that --kubeconfig also reads the DHCTL_CLI_KUBE_CONFIG environment variable. "+
					"Unset one of them and run destroy again",
				source,
			)
		}

		cacheIdentity := ""
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
		if opts.Kube.InCluster {
			// No conflict with --kubeconfig to handle here: lib-connection's
			// Config.IsConflict rejects two set modes while the providers are built
			// above, and that error is returned at once.
			identity, err := inClusterCacheIdentity()
			if err != nil {
				return err
			}
			cacheIdentity = identity
		}
		if sshHostConfigured {
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
		if cacheIdentity == "" {
			return errors.New("nothing identifies this cluster: pass --ssh-host for a cluster with SSH, or --kubeconfig for one without")
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

// kubeconfigClusterIdentity names the cluster a kubeconfig points at, from what
// is inside it rather than from where it sits: the API server address and the
// cluster CA. Two clusters cannot share both.
//
// The CA has to be there. The address alone identifies nothing here — the line
// this command prints for an immutable cluster tells every operator to point
// their kubeconfig at https://127.0.0.1:6445 through a bastion forward, so
// "same address, different cluster" is the normal shape and not the exotic one.
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

	// certificate-authority is a path and is left as one by LoadFromFile, so an
	// unread file would leave the address hashed on its own — the very
	// collision this function exists to prevent, and a silent one.
	certificateAuthority := cluster.CertificateAuthorityData
	if len(certificateAuthority) == 0 && cluster.CertificateAuthority != "" {
		caPath := cluster.CertificateAuthority
		if !filepath.IsAbs(caPath) {
			caPath = filepath.Join(filepath.Dir(path), caPath)
		}
		certificateAuthority, err = os.ReadFile(caPath)
		if err != nil {
			return "", fmt.Errorf("read the cluster CA %s: %w", caPath, err)
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

	// The CA alone, deliberately not the server address: this same command prints
	// a sed line telling the operator to rewrite that address to
	// https://127.0.0.1:6445 for a bastion forward, so hashing it would give an
	// interrupted destroy a different cache directory when it is resumed through
	// the tunnel — an empty cache, and no infrastructure state to finish from. A
	// cluster CA is unique to its cluster, which is all this has to establish.
	h := sha256.New()
	h.Write(certificateAuthority)
	return "kubeconfig-" + hex.EncodeToString(h.Sum(nil)), nil
}
