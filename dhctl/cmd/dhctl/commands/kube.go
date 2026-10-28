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
	"os"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func DefineTestKubernetesAPIConnectionCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		doneCh := make(chan struct{})
		tomb.RegisterOnShutdown("wait for kubernetes-api-connection to stop", func() {
			<-doneCh
		})

		checker := controlplane.NewKubeProxyChecker().
			WithLogResult(true).
			WithAskPassword(true).
			WithInitParams(client.AppKubernetesInitParams(&opts.Kube))

		proxyClose := func() {
			l.Info("Press Ctrl+C to close the proxy connection.")
			ch := make(chan struct{})
			<-ch
		}

		// ip is empty because we want check via ssh-hosts passed via cm args
		ready, err := checker.IsReady(ctx, "")
		if err != nil {
			proxyClose()
			return err
		}

		if !ready {
			proxyClose()
			return fmt.Errorf("Proxy not ready")
		}

		TestCommandDelay()
		close(doneCh)

		return nil
	}
}

func DefineWaitDeploymentReadyCommand(cmd *cobra.Command, opts *options.Options) {
	app.DefineSSHFlags(cmd, opts)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	var Namespace string
	var Name string

	app.StringFlag(cmd.Flags(), app.Flag{Name: "namespace", Help: "Use namespace"}, &Namespace)
	app.StringFlag(cmd.Flags(), app.Flag{Name: "name", Help: "Deployment name"}, &Name)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		l := logger.FromContext(ctx)

		sshProviderInitializer, kubeProvider, err := providerinitializer.CLIProviders(ctx, opts, providerinitializer.CLIProvidersOptions{RequireKube: true})
		if err != nil {
			return err
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}

		return logger.RunProcess(ctx, l, "Wait for Deckhouse to become Ready", func(ctx context.Context) error {
			kube, err := kubeProvider.Client(ctx)
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %w", err)
			}
			kubeCl := &client.KubernetesClient{KubeClient: kube}

			return deckhouse.WaitForReadiness(ctx, kubeCl, opts.Bootstrap.DeckhouseTimeout)
		})
	}
}

func TestCommandDelay() {
	delayStr := os.Getenv("TEST_DELAY")
	if delayStr == "" || delayStr == "no" {
		return
	}

	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		delay = time.Minute
	}

	time.Sleep(delay)
}
