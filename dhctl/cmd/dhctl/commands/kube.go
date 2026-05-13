// Copyright 2021 Flant JSC
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

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func DefineTestKubernetesAPIConnectionCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		doneCh := make(chan struct{})
		tomb.RegisterOnShutdown("wait kubernetes-api-connection to stop", func() {
			<-doneCh
		})

		interactive := input.IsTerminal()
		if interactive {
			onComplete, _, err := progressbar.InitProgressBarWithDeferredFunc("test Kubernetes API connection", log.GetDefaultLogger())
			if err != nil {
				return err
			}
			defer onComplete()
		}

		checker := controlplane.NewKubeProxyChecker().
			WithLogResult(true).
			WithAskPassword(true).
			WithInitParams(client.AppKubernetesInitParams(&opts.Kube))

		proxyClose := func() {
			log.InfoLn("Press Ctrl+C to close proxy connection.")
			if interactive {
				progressbar.InfoF("%s\n", "Press Ctrl+C to close proxy connection.")
			}
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
	})
}

func DefineWaitDeploymentReadyCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)

	var Namespace string
	var Name string

	cmd.Flag("namespace", "Use namespace").
		StringVar(&Namespace)
	cmd.Flag("name", "Deployment name").
		StringVar(&Name)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		loggerProvider := log.ExternalLoggerProvider(logger)
		params := app.ProviderParams(&opts.Global, loggerProvider)
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}

		if input.IsTerminal() {
			onComplete, _, err := progressbar.InitProgressBarWithDeferredFunc("Wait for deployment is Ready", logger)
			if err != nil {
				return err
			}
			defer onComplete()
		}

		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		return log.ProcessCtx(ctx, "bootstrap", "Wait for Deckhouse to become Ready", func(ctx context.Context) error {
			kube, err := kubeProvider.Client(ctx)
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %w", err)
			}
			kubeCl := &client.KubernetesClient{KubeClient: kube}

			return deckhouse.WaitForReadiness(ctx, kubeCl, opts.Bootstrap.DeckhouseTimeout)
		})
	})
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
