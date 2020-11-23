package commands

import (
	"encoding/json"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/converge"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/terraform"
	"flant/candictl/pkg/util/cache"
	"flant/candictl/pkg/util/retry"
	"flant/candictl/pkg/util/tomb"
)

func getClientOnce(sshClient *ssh.SSHClient, kubeCl *client.KubernetesClient) (*client.KubernetesClient, error) {
	var err error
	if kubeCl == nil {
		kubeCl, err = operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return nil, err
		}

		if info := deckhouse.GetClusterInfo(kubeCl); info != "" {
			_ = log.Process("common", "Cluster Info", func() error { log.InfoF(info); return nil })
		}
	}
	return kubeCl, err
}

const (
	destroyCacheErrorMessage = `Create cache:
	Error: %v

	Probably that Kubernetes cluster was already deleted.
	If you want to continue, please delete the cache folder manually.
`
	destroyApprovalsMessage = `You will be asked for approve multiple times.
If you understand what you are doing, you can use flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.

`
)

func DefineDestroyCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("destroy", "Destroy Kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTerraformFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineSkipResourcesFlags(cmd)

	runFunc := func(sshClient *ssh.SSHClient) error {
		var err error

		stateCache, err := cache.NewTempStateCache(sshClient.Check().String())
		if err != nil {
			return fmt.Errorf(destroyCacheErrorMessage, err)
		}

		var kubeCl *client.KubernetesClient
		if !app.SkipResources {
			if kubeCl, err = getClientOnce(sshClient, kubeCl); err != nil {
				return err
			}

			err = log.Process("common", "Delete resources from the Kubernetes cluster", func() error {
				return deleteEntities(kubeCl)
			})
			if err != nil {
				return err
			}
		}

		var metaConfig *config.MetaConfig
		if stateCache.InCache("cluster-config") && retry.AskForConfirmation("Do you want to continue with Cluster configuration from local cache") {
			if err := stateCache.LoadStruct("cluster-config", &metaConfig); err != nil {
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

			metaConfig.UUID, err = converge.GetClusterUUID(kubeCl)
			if err != nil {
				return err
			}

			err := stateCache.SaveStruct("cluster-config", metaConfig)
			if err != nil {
				return err
			}
		}
		stateCache.AddToClean("cluster-config")

		var nodesState map[string]converge.NodeGroupTerraformState
		if stateCache.InCache("nodes-state") && retry.AskForConfirmation("Do you want to continue with Nodes state from local cache") {
			if err := stateCache.LoadStruct("nodes-state", &nodesState); err != nil {
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
			err := stateCache.SaveStruct("nodes-state", nodesState)
			if err != nil {
				return err
			}
		}
		stateCache.AddToClean("nodes-state")

		var clusterState []byte
		if stateCache.InCache("cluster-state") && retry.AskForConfirmation("Do you want to continue with Cluster state from local cache") {
			clusterState = stateCache.Load("cluster-state")
		} else {
			if kubeCl, err = getClientOnce(sshClient, kubeCl); err != nil {
				return err
			}
			clusterState, err = converge.GetClusterStateFromCluster(kubeCl)
			if err != nil {
				return err
			}
			stateCache.Save("cluster-state", clusterState)
		}
		stateCache.AddToClean("cluster-state")

		// Stop proxy because we have already gotten all info from kubernetes-api
		if kubeCl != nil {
			kubeCl.KubeProxy.Stop()
		}

		for nodeGroupName, nodeGroupStates := range nodesState {
			cfg, err := metaConfig.DeepCopy().Prepare()
			if err != nil {
				return fmt.Errorf("unable to prepare copied config: %v", err)
			}
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
				if !stateCache.InCache(name) {
					stateCache.Save(name, state)
				}

				nodeRunner := terraform.NewRunnerFromConfig(metaConfig, step).
					WithVariables(metaConfig.NodeGroupConfig(nodeGroupName, 0, "")).
					WithCache(stateCache).
					WithStatePath(stateCache.ObjectPath(name)).
					WithAutoApprove(app.SanityCheck)

				tomb.RegisterOnShutdown(nodeRunner.Stop)

				err := terraform.DestroyPipeline(nodeRunner, name)
				if err != nil {
					return fmt.Errorf("destroing of node %s failed: %v", name, err)
				}
			}
		}

		if !stateCache.InCache("base-infrastructure") {
			stateCache.Save("base-infrastructure", clusterState)
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
			WithVariables(metaConfig.MarshalConfig()).
			WithCache(stateCache).
			WithStatePath(stateCache.ObjectPath("base-infrastructure")).
			WithAutoApprove(app.SanityCheck)
		tomb.RegisterOnShutdown(baseRunner.Stop)

		if err = terraform.DestroyPipeline(baseRunner, "Kubernetes cluster"); err != nil {
			return err
		}

		stateCache.Clean()
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.Warning(destroyApprovalsMessage)
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}
		if err := operations.AskBecomePassword(); err != nil {
			return err
		}

		return runFunc(sshClient)
	})
	return cmd
}

func deleteEntities(kubeCl *client.KubernetesClient) error {
	err := deckhouse.DeleteDeckhouseDeployment(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteServices(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForServicesDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteStorageClasses(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePVC(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePV(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePods(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVCDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteMachinesIfResourcesExist(kubeCl)
	if err != nil {
		return err
	}

	return nil
}
