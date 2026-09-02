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
	"fmt"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// kubeProviderThroughBastion replaces the Kubernetes provider with one that
// reaches the API through the bastion, for a cluster whose nodes run no sshd and
// whose kubeconfig therefore names an address only the cluster network resolves.
// Returns the given provider unchanged, and a no-op closer, for every other run.
func kubeProviderThroughBastion(
	ctx context.Context,
	opts *options.Options,
	sshProviderInitializer *providerinitializer.SSHProviderInitializer,
	kubeProvider libcon.KubeProvider,
) (libcon.KubeProvider, func(), error) {
	noop := func() {}

	if opts.Kube.Config == "" {
		return kubeProvider, noop, nil
	}
	if sshProviderInitializer == nil {
		return kubeProvider, noop, nil
	}
	if immutable.BastionConfig(sshProviderInitializer.GetConfig()) == nil {
		return kubeProvider, noop, nil
	}
	// An SSH host means the operator named a machine to work through, and that
	// machine reaches the API itself. Only a run with no host at all is one where
	// the kubeconfig is the sole way in.
	if sshProviderInitializer.CheckHosts(ctx) {
		return kubeProvider, noop, nil
	}

	path, stop, err := immutable.OpenKubeconfigChannel(
		ctx,
		sshProviderInitializer.GetConfig(),
		sshProviderInitializer.GetSettings(),
		opts.Kube.Config,
		opts.Kube.ConfigContext,
		opts.Global.TmpDir,
	)
	if err != nil {
		return nil, noop, err
	}

	// Mirrors newKubeconfigKubeProvider of pkg/operations/bootstrap:
	// a kubeconfig-backed client needs no SSH runner, hence the nil.
	kubeConfig := &kube.Config{KubeConfig: path, KubeConfigContext: opts.Kube.ConfigContext}
	runner, err := provider.GetRunnerInterface(ctx, kubeConfig, sshProviderInitializer.GetSettings(), nil)
	if err != nil {
		stop()
		return nil, noop, fmt.Errorf("build the Kubernetes runner interface: %w", err)
	}

	return provider.NewDefaultKubeProvider(sshProviderInitializer.GetSettings(), kubeConfig, runner), stop, nil
}
