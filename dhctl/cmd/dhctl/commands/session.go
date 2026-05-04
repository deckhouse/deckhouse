// Copyright 2025 Flant JSC
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
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const (
	clusterName = "local"
	contextName = "local"
	userName    = "local"
)

func DefineSessionCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, nil)
	app.DefineBecomeFlags(cmd)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		params, err := app.DefaultProviderParams()
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
		defer sshProviderInitializer.Cleanup(ctx)

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
		if err := localKubeConfig(apiServerURL); err != nil {
			return fmt.Errorf("error save kubeconfig: %v", err)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		// todo(log): why do not use logger?
		fmt.Println("Received signal:", sig)
		fmt.Println("Exiting SSH tunnel...")

		return nil
	})
}

func localKubeConfig(apiServerURL string) error {
	kubeconfigDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to open home directory: %w", err)
	}

	kubeconfigPath := filepath.Join(kubeconfigDir, ".kube", "config")
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0755); err != nil {
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

	// todo(log): why do not use logger?
	fmt.Printf("Kubeconfig successfully saved at: %s\n", kubeconfigPath)

	return nil
}
