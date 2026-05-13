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
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
)

func DefineTestControlPlaneManagerReadyCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineControlPlaneFlags(cmd, &opts.ControlPlane, false)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		loggerProvider := log.ExternalLoggerProvider(logger)
		params := app.ProviderParams(&opts.Global, loggerProvider)

		interactive := input.IsTerminal()
		if interactive {
			onComplete, _, err := progressbar.InitProgressBarWithDeferredFunc("Test control plane manager is Ready", logger)
			if err != nil {
				return err
			}
			defer onComplete()
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}
		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %w", err)
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		checker := controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl))
		ready, err := checker.IsReady(ctx, opts.ControlPlane.Hostname)
		if err != nil {
			return fmt.Errorf("Control plane manager is not ready: %s", err)
		}

		if ready {
			log.InfoLn("Control plane manager is ready")
			if interactive {
				progressbar.InfoF("%s\n", "Control plane manager is ready")
			}
		} else {
			log.WarnLn("Control plane manager is not ready")
			if interactive {
				progressbar.WarnF("%s\n", "Control plane manager is not ready")
			}
		}

		return nil
	})
}

func DefineTestControlPlaneNodeReadyCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)
	app.DefineKubeFlags(cmd, &opts.Kube)
	app.DefineControlPlaneFlags(cmd, &opts.ControlPlane, true)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)
		logger := log.GetDefaultLogger()

		loggerProvider := log.ExternalLoggerProvider(logger)
		params := app.ProviderParams(&opts.Global, loggerProvider)

		interactive := input.IsTerminal()
		if interactive {
			onComplete, _, err := progressbar.InitProgressBarWithDeferredFunc("Test control plane node is Ready", logger)
			if err != nil {
				return err
			}
			defer onComplete()
		}

		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}
		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx)
		}

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %w", err)
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		nodeToHostForChecks := map[string]string{opts.ControlPlane.Hostname: opts.ControlPlane.IP}

		checkers := []hook.NodeChecker{hook.NewKubeNodeReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl))}

		if opts.ControlPlane.Hostname != "" {
			checkers = append(checkers, controlplane.NewKubeProxyChecker().WithExternalIPs(nodeToHostForChecks))
		}

		checkers = append(checkers, controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl)))

		err = controlplane.NewChecker(nodeToHostForChecks, checkers, "test", controlplane.DefaultConfirm).
			IsAllNodesReady(ctx)
		if err != nil {
			return fmt.Errorf("control plane node is not ready: %v", err)
		}

		log.InfoLn("Control plane manager node is ready")
		if interactive {
			progressbar.InfoF("%s\n", "Control plane manager node is ready")
		}

		return nil
	})
}
