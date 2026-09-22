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

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// destroyCacheIdentity names the cluster whose infrastructure state destroy will
// delete, and the SSH provider it reaches it through when there is one. A wrong
// identity silently addresses somebody else's state.
func destroyCacheIdentity(
	ctx context.Context,
	opts *options.Options,
	sshProviderInitializer *providerinitializer.SSHProviderInitializer,
) (string, libcon.SSHProvider, error) {
	sshHostConfigured := sshProviderInitializer != nil && sshProviderInitializer.CheckHosts(ctx)
	if err := refuseTwoClusterSources(opts, sshHostConfigured); err != nil {
		return "", nil, err
	}

	// One source names the cluster, and the checks above have already refused the
	// combinations that would let two of them disagree.
	switch {
	case sshHostConfigured:
		provider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return "", nil, err
		}
		sshClient, err := provider.Client(ctx)
		if err != nil {
			return "", nil, err
		}
		return sshClient.Check().String(), provider, nil

	case opts.Kube.InCluster:
		// No conflict with --kubeconfig to handle here: lib-connection's
		// Config.IsConflict rejects two set modes while the providers are built by
		// the caller, and that error is returned at once.
		identity, err := inClusterCacheIdentity()
		if err != nil {
			return "", nil, err
		}
		return identity, nil, nil

	case opts.Kube.Config != "":
		// NOT GetCacheIdentityFromKubeconfig: that hashes the path, and every
		// immutable bootstrap writes to the same path — all clusters on this
		// machine would then share the cache destroy reads its state from.
		identity, err := kubeconfigClusterIdentity(opts.Kube.Config, opts.Kube.ConfigContext)
		if err != nil {
			return "", nil, fmt.Errorf("identify the cluster from %s: %w", opts.Kube.Config, err)
		}
		return identity, nil, nil

	default:
		return "", nil, errors.New("nothing identifies this cluster: pass --ssh-host for a cluster with SSH, or --kubeconfig for one without")
	}
}

// refuseTwoClusterSources rejects a kubeconfig and an SSH host naming a cluster
// together rather than ranking them: they may be different clusters, and
// --kubeconfig also reads DHCTL_CLI_KUBE_CONFIG.
func refuseTwoClusterSources(opts *options.Options, sshHostConfigured bool) error {
	kubeSourceConfigured := opts.Kube.Config != "" || opts.Kube.InCluster
	if !sshHostConfigured || !kubeSourceConfigured {
		return nil
	}

	// Named before either source is read, so the operator hears about the
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
// inside it: its CA and the admin certificate issued against that CA. The address
// identifies nothing — the bastion-forward line retargets every cluster to :6445.
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
	certificateAuthority, err := certificateBytes(cluster.CertificateAuthorityData, cluster.CertificateAuthority)
	if err != nil {
		return "", fmt.Errorf("read the cluster CA: %w", err)
	}
	if len(certificateAuthority) == 0 {
		return "", fmt.Errorf(
			"context %q carries no cluster CA (certificate-authority or certificate-authority-data): "+
				"its server address alone does not tell this cluster from another one reached at the same address, "+
				"and destroy reads the infrastructure state out of a cache keyed by that answer",
			contextName,
		)
	}

	// One PKI can back several clusters — a DR clone, a rebuild from an etcd
	// snapshot — and one cache directory for two of them is destroy deleting the
	// other one's infrastructure. The admin certificate is issued once per cluster.
	adminCertificate, err := adminCertificateBytes(cfg, kubeContext.AuthInfo)
	if err != nil {
		return "", err
	}

	// The server address is still left out: a destroy resumed through the bastion
	// forward carries a rewritten one, so hashing it would hand the resume an empty
	// cache with no infrastructure state to finish from.
	identity := sha256.New()
	identity.Write(certificateAuthority)
	identity.Write(adminCertificate)
	return fmt.Sprintf("kubeconfig-%x", identity.Sum(nil)), nil
}

// adminCertificateBytes returns the client certificate the context's user
// authenticates with, or nothing when it authenticates some other way — a token
// or an exec plugin says nothing about which cluster it belongs to.
func adminCertificateBytes(cfg *clientcmdapi.Config, authInfoName string) ([]byte, error) {
	authInfo, found := cfg.AuthInfos[authInfoName]
	if !found {
		return nil, nil
	}

	certificate, err := certificateBytes(authInfo.ClientCertificateData, authInfo.ClientCertificate)
	if err != nil {
		return nil, fmt.Errorf("read the client certificate of user %q: %w", authInfoName, err)
	}
	return certificate, nil
}

// certificateBytes returns an embedded certificate, or the contents of the file
// the kubeconfig names instead. ResolveLocalPaths has already resolved that path
// against the kubeconfig's own directory.
func certificateBytes(data []byte, path string) ([]byte, error) {
	if len(data) > 0 {
		return data, nil
	}
	if path == "" {
		return nil, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return content, nil
}
