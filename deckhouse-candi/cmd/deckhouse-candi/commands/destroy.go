package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
	"flant/deckhouse-candi/pkg/terraform"
	"flant/deckhouse-candi/pkg/util/cache"
	"flant/deckhouse-candi/pkg/util/retry"
)

func getClientOnce(sshClient *ssh.SshClient, kubeCl *client.KubernetesClient) (*client.KubernetesClient, error) {
	var err error
	if kubeCl == nil {
		kubeCl, err = commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return nil, err
		}
	}
	return kubeCl, err
}

func DefineDestroyCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("destroy", "Destroy Kubernetes cluster.")
	app.DefineSshFlags(cmd)
	app.DefineTerraformFlags(cmd)
	app.DefineSanityFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		err := app.AskBecomePassword()
		if err != nil {
			return err
		}

		if err = cache.Init(sshClient.Check().String()); err != nil {
			return fmt.Errorf(
				"Create cache:\n\tError: %v\n\n"+
					"\tProbably that Kubernetes cluster was already deleted.\n"+
					"\tIf you want to continue, please delete the cache folder manually.",
				err,
			)
		}

		var kubeCl *client.KubernetesClient

		var metaConfig *config.MetaConfig
		if cache.Global().InCache("cluster-config") && retry.AskForConfirmation("Do you want to continue with Cluster configuration from local cash") {
			if err := cache.Global().LoadStruct("cluster-config", &metaConfig); err != nil {
				return err
			}
		} else {
			if kubeCl, err = getClientOnce(sshClient, kubeCl); err != nil {
				return err
			}
			metaConfig, err = config.ParseConfigFromCluster(kubeCl)
			if err != nil {
				return err
			}
			err := cache.Global().SaveStruct("cluster-config", metaConfig)
			if err != nil {
				return err
			}
		}
		cache.Global().AddToClean("cluster-config")

		var nodesState map[string]converge.NodeGroupTerraformState
		if cache.Global().InCache("nodes-state") && retry.AskForConfirmation("Do you want to continue with Nodes state from local cash") {
			if err := cache.Global().LoadStruct("nodes-state", &nodesState); err != nil {
				return err
			}
		} else {
			if kubeCl, err = getClientOnce(sshClient, kubeCl); err != nil {
				return err
			}
			nodesState, err = converge.GetNodesStateFromCluster(kubeCl)
			if err != nil {
				return err
			}
			err := cache.Global().SaveStruct("nodes-state", nodesState)
			if err != nil {
				return err
			}
		}
		cache.Global().AddToClean("nodes-state")

		var clusterState []byte
		if cache.Global().InCache("cluster-state") && retry.AskForConfirmation("Do you want to continue with Cluster state from local cash") {
			clusterState = cache.Global().Load("cluster-state")
		} else {
			if kubeCl, err = getClientOnce(sshClient, kubeCl); err != nil {
				return err
			}
			clusterState, err = converge.GetClusterStateFromCluster(kubeCl)
			if err != nil {
				return err
			}
			cache.Global().Save("cluster-state", clusterState)
		}
		cache.Global().AddToClean("cluster-state")

		for nodeGroupName, nodeGroupStates := range nodesState {
			cfg := metaConfig.DeepCopy().Prepare()
			if nodeGroupStates.Settings != nil {
				nodeGroupsSettings, err := json.Marshal([]json.RawMessage{nodeGroupStates.Settings})
				if err != nil {
					log.ErrorLn(err)
				} else {
					cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
				}
			}

			step := "static-node"
			if nodeGroupName == "master" {
				step = "master-node"
			}

			for name, state := range nodeGroupStates.State {
				nodeRunner := terraform.NewRunnerFromConfig(metaConfig, step).
					WithVariables(metaConfig.NodeGroupConfig(nodeGroupName, 0, "")).
					WithState(state).
					WithAutoApprove(app.SanityCheck)

				err := terraform.DestroyPipeline(nodeRunner, name)
				if err != nil {
					log.ErrorLn(err)
					log.ErrorLn("Maybe the node has already been removed.")
					// We need to skip error there, because we don't modify data in cache
					// even if node had been already deleted
				}

				nodeRunner.Close()
			}
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
			WithVariables(metaConfig.MarshalConfig()).
			WithState(clusterState).
			WithAutoApprove(app.SanityCheck)

		defer baseRunner.Close()

		if err = terraform.DestroyPipeline(baseRunner, "Kubernetes cluster"); err != nil {
			return err
		}

		cache.Global().Clean()
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.Warning("You will be asked for approve multiple times.\n" +
				"If you understand what you are doing, you can use flag " +
				"--yes-i-am-sane-and-i-understand-what-i-am-doing to skip approvals.\n\n")
		}
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = runFunc(sshClient)
		if err != nil {
			log.ErrorLn(err.Error())
			os.Exit(1)
		}
		return nil
	})
	return cmd
}
