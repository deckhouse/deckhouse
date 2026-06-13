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
	"os/signal"
	"path/filepath"
	"syscall"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const (
	clusterName = "local"
	contextName = "local"
	userName    = "local"
)

func DefineSessionCommand(cmd *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, &opts.SSH, nil)
	app.DefineBecomeFlags(cmd, &opts.Become)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		params, err := app.DefaultProviderParams(ctx, &opts.Global)
		if err != nil {
			return err
		}
		sshProviderInitializer, err := providerinitializer.GetSSHProviderInitializer(ctx, params)
		if err != nil {
			return err
		}

		if sshProviderInitializer == nil {
			return fmt.Errorf("Not enough flags were provided to perform the operation.\nUse dhctl session --help to get available flags.")
		}

		defer providerinitializer.CleanupSSHProvider(ctx, sshProviderInitializer)

		sshProvider, err := sshProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshCl, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}
		apiServerPort, err := sshCl.KubeProxy().Start(-1)
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %w", err)
		}

		apiServerURL := fmt.Sprintf("http://localhost:%s", apiServerPort)
		if err := localKubeConfig(ctx, apiServerURL); err != nil {
			return fmt.Errorf("save kubeconfig: %v", err)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		l := dhlog.FromContext(ctx)
		l.InfoContext(ctx, fmt.Sprintf("Received signal: %v", sig), dhlog.ShowInCompacted())
		l.InfoContext(ctx, "Exiting SSH tunnel...", dhlog.ShowInCompacted())

		return nil
	})
}

func localKubeConfig(ctx context.Context, apiServerURL string) error {
	kubeconfigDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to open home directory: %w", err)
	}

	kubeconfigPath := filepath.Join(kubeconfigDir, ".kube", "config")
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	kubeConfig := api.NewConfig()
	kubeConfig.Clusters[clusterName] = &api.Cluster{
		Server:                apiServerURL,
		InsecureSkipTLSVerify: true,
	}
	kubeConfig.AuthInfos[userName] = &api.AuthInfo{}
	kubeConfig.Contexts[contextName] = &api.Context{
		Cluster:  clusterName,
		AuthInfo: userName,
	}
	kubeConfig.CurrentContext = contextName

	if err := clientcmd.WriteToFile(*kubeConfig, kubeconfigPath); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	dhlog.FromContext(ctx).InfoContext(ctx, "Kubeconfig successfully saved at: "+kubeconfigPath, dhlog.ShowInCompacted())

	return nil
}
