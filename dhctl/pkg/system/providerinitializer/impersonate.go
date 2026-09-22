// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/lib-connection/pkg/kube"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// impersonationProbeTimeout bounds the single request that asks whether this
// kubeconfig may impersonate: an API server that is not up yet must not hold up a
// command that retries the connection on its own anyway.
const impersonationProbeTimeout = 15 * time.Second

// impersonateKubeConfig makes a kubeconfig-backed connection act as
// global.ImpersonateUser in global.ImpersonateGroup, the identity the kubectl
// proxy on a master is started with. Deckhouse's admission policies exempt that
// username and nothing else, so without this a --kubeconfig run is denied where
// the SSH path is allowed - deleting a CAPI Cluster on destroy, for one.
func impersonateKubeConfig(ctx context.Context, cfg *kube.Config) (*kube.Config, error) {
	if cfg.KubeConfig == "" {
		return cfg, nil
	}

	restConfig, err := kubeconfigRESTConfig(cfg.KubeConfig, cfg.KubeConfigContext)
	if err != nil {
		return nil, err
	}
	restConfig.Impersonate = rest.ImpersonationConfig{
		UserName: global.ImpersonateUser,
		Groups:   []string{global.ImpersonateGroup},
	}

	if impersonationRefused(ctx, restConfig) {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"The user of kubeconfig %s may not impersonate, so dhctl acts as that user itself. "+
				"Operations Deckhouse allows only to %q, such as deleting a Cluster resource, will be denied.",
			cfg.KubeConfig, global.ImpersonateUser,
		))
		return cfg, nil
	}

	// The kubeconfig turns into a rest config whole: lib-connection reads the two
	// as separate modes and rejects a config that sets both (kube.Config.IsConflict).
	return &kube.Config{RestConfig: restConfig}, nil
}

// kubeconfigRESTConfig builds the client configuration a kubeconfig describes the
// same way the client below lib-connection does (flant/kube-client
// client.getClientConfig), so that handing over the rest config instead of the
// path changes the identity and nothing else.
func kubeconfigRESTConfig(path, contextName string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	rules.ExplicitPath = path

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build the client configuration from kubeconfig %s: %w", path, err)
	}

	return restConfig, nil
}

// impersonationRefused reports whether the API server denies this kubeconfig's own
// user the right to impersonate. Every request carries that check, so /version is
// enough, and a 403 can only be about the impersonation itself: the impersonated
// identity is system:masters, which no authorizer can deny. Anything else - an
// unreachable server, a 401, a TLS failure - is not an answer about impersonation
// and keeps it on, because the connection is retried later anyway.
func impersonationRefused(ctx context.Context, restConfig *rest.Config) bool {
	probeConfig := rest.CopyConfig(restConfig)
	probeConfig.Timeout = impersonationProbeTimeout

	client, err := discovery.NewDiscoveryClientForConfig(probeConfig)
	if err != nil {
		return false
	}

	_, err = client.RESTClient().Get().AbsPath("/version").DoRaw(ctx)
	return apierrors.IsForbidden(err)
}
