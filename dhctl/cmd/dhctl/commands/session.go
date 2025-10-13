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
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

const (
	clusterName = "local"
	contextName = "local"
	userName    = "local"
)

func DefineSessionCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	app.DefineSSHFlags(cmd, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		if sshClient == nil {
			return fmt.Errorf("Not enough flags were provided to perform the operation.\nUse dhctl session --help to get available flags.")
		}

		kubeCl := client.NewKubernetesClient().WithNodeInterface(ssh.NewNodeInterfaceWrapper(sshClient))
		apiServerPort, err := kubeCl.StartKubernetesProxy(context.Background())
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}
		apiServerURL := fmt.Sprintf("http://localhost:%s", apiServerPort)

		err = localKubeConfig(apiServerURL)
		if err != nil {
			return fmt.Errorf("error save kubeconfig: %v", err)
		}
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigChan:
			fmt.Println("Received signal:", sig)
			fmt.Println("Exiting SSH tunnel...")
		}
		return nil
	})
	return cmd
}

func localKubeConfig(apiServerUrl string) error {
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
		Server:                apiServerUrl,
		InsecureSkipTLSVerify: true,
	}
	kubeConfig.AuthInfos[userName] = &api.AuthInfo{}
	kubeConfig.Contexts[contextName] = &api.Context{
		Cluster:  clusterName,
		AuthInfo: userName,
	}
	kubeConfig.CurrentContext = contextName

	err = clientcmd.WriteToFile(*kubeConfig, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	fmt.Printf("Kubeconfig successfully saved at: %s\n", kubeconfigPath)
	return nil
}
