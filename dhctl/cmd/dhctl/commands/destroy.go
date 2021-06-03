package commands

import (
	"encoding/json"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

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

type k8sClientGetter struct {
	convergeLock *client.LeaseLock
	sshClient    *ssh.Client
	kubeCl       *client.KubernetesClient
}

func newK8sClientGetter(sshClient *ssh.Client) *k8sClientGetter {
	return &k8sClientGetter{
		sshClient: sshClient,
	}
}

func (g *k8sClientGetter) convergeUnlock(onlyNil bool) {
	if onlyNil {
		g.convergeLock = nil
		return
	}

	if g.convergeLock != nil {
		g.convergeLock.Unlock()
		g.convergeLock = nil
	}
}

func (g *k8sClientGetter) get() (*client.KubernetesClient, error) {
	if g.kubeCl != nil {
		return g.kubeCl, nil
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(g.sshClient)
	if err != nil {
		return nil, err
	}

	destroyerIdentity := config.GetLocalConvergeLockIdentity("local-destroyer")
	leaseConfig := config.GetConvergeLockLeaseConfig(destroyerIdentity)
	convergeLock := client.NewLeaseLock(kubeCl, leaseConfig)
	err = convergeLock.Lock()
	if err != nil {
		return nil, err
	}

	if info := deckhouse.GetClusterInfo(kubeCl); info != "" {
		_ = log.Process("common", "Cluster Info", func() error { log.InfoF(info); return nil })
	}

	g.kubeCl = kubeCl
	g.convergeLock = convergeLock

	return kubeCl, err
}

func deleteResources(k8sCliGetter *k8sClientGetter) error {
	if app.SkipResources {
		return nil
	}

	kubeCl, err := k8sCliGetter.get()
	if err != nil {
		return err
	}

	return log.Process("common", "Delete resources from the Kubernetes cluster", func() error {
		return deleteEntities(kubeCl)
	})
}

func loadMetaConfig(k8sCliGetter *k8sClientGetter, stateCache *cache.StateCache) (*config.MetaConfig, error) {
	var metaConfig *config.MetaConfig
	var err error

	confirmation := input.NewConfirmation().
		WithMessage("Do you want to continue with Cluster configuration from local cache?").
		WithYesByDefault()
	if stateCache.InCache("cluster-config") && confirmation.Ask() {
		if err := stateCache.LoadStruct("cluster-config", &metaConfig); err != nil {
			return nil, err
		}
		return metaConfig, nil
	}

	kubeCl, err := k8sCliGetter.get()
	if err != nil {
		return nil, err
	}

	metaConfig, err = config.ParseConfigFromCluster(kubeCl)
	if err != nil {
		return nil, err
	}

	metaConfig.UUID, err = converge.GetClusterUUID(kubeCl)
	if err != nil {
		return nil, err
	}

	if err := stateCache.SaveStruct("cluster-config", metaConfig); err != nil {
		return nil, err
	}
	return metaConfig, nil
}

func DefineDestroyCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("destroy", "Destroy Kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineSkipResourcesFlags(cmd)

	runFunc := func(sshClient *ssh.Client) error {
		k8sCliGetter := newK8sClientGetter(sshClient)
		defer k8sCliGetter.convergeUnlock(false)

		var err error

		stateCache, err := cache.NewTempStateCache(sshClient.Check().String())
		if err != nil {
			return fmt.Errorf(destroyCacheErrorMessage, err)
		}

		var kubeCl *client.KubernetesClient

		if err := deleteResources(k8sCliGetter); err != nil {
			return err
		}

		metaConfig, err := loadMetaConfig(k8sCliGetter, stateCache)
		if err != nil {
			return err
		}

		var nodesState map[string]converge.NodeGroupTerraformState
		confirmation := input.NewConfirmation().
			WithMessage("Do you want to continue with Nodes state from local cache?").
			WithYesByDefault()

		if stateCache.InCache("nodes-state") && confirmation.Ask() {
			if err := stateCache.LoadStruct("nodes-state", &nodesState); err != nil {
				return err
			}
		} else {
			if kubeCl, err = k8sCliGetter.get(); err != nil {
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

		var clusterState []byte
		confirmation = input.NewConfirmation().
			WithMessage("Do you want to continue with Cluster state from local cache?").
			WithYesByDefault()

		if stateCache.InCache("cluster-state") && confirmation.Ask() {
			clusterState = stateCache.Load("cluster-state")
			if len(clusterState) == 0 {
				return fmt.Errorf("can't load cluster state from cache")
			}
		} else {
			if kubeCl, err = k8sCliGetter.get(); err != nil {
				return err
			}
			clusterState, err = converge.GetClusterStateFromCluster(kubeCl)
			if err != nil {
				return err
			}
			stateCache.Save("cluster-state", clusterState)
		}

		// why only nil lock without request unlock
		// user may not delete resources and converge still working in cluster
		// all node groups removing may still in long time run and
		// we get race (destroyer destroy node group, auto applayer create nodes)
		k8sCliGetter.convergeUnlock(true)
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
				stateName := fmt.Sprintf("%s.tfstate", name)
				if !stateCache.InCache(stateName) {
					stateCache.Save(stateName, state)
				}

				nodeRunner := terraform.NewRunnerFromConfig(metaConfig, step).
					WithVariables(metaConfig.NodeGroupConfig(nodeGroupName, 0, "")).
					WithName(name).
					WithCache(stateCache).
					WithAllowedCachedState(true).
					WithAutoApprove(app.SanityCheck)

				tomb.RegisterOnShutdown(name, nodeRunner.Stop)

				err := terraform.DestroyPipeline(nodeRunner, name)
				if err != nil {
					return fmt.Errorf("destroing of node %s failed: %v", name, err)
				}
			}
		}

		if !stateCache.InCache("base-infrastructure.tfstate") {
			stateCache.Save("base-infrastructure.tfstate", clusterState)
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
			WithVariables(metaConfig.MarshalConfig()).
			WithCache(stateCache).
			WithAllowedCachedState(true).
			WithAutoApprove(app.SanityCheck)
		tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

		if err = terraform.DestroyPipeline(baseRunner, "Kubernetes cluster"); err != nil {
			return err
		}

		stateCache.Clean()
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.WarnLn(destroyApprovalsMessage)
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}
		if err := terminal.AskBecomePassword(); err != nil {
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

	err = deckhouse.WaitForDeckhouseDeploymentDeletion(kubeCl)
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
