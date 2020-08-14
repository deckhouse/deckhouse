package converge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/terraform"
)

const masterNodeGroupName = "master"

func BootstrapAdditionalNode(kubeCl *client.KubernetesClient, index int, providerName, layout, step, nodeGroupName, cloudConfig string, metaConfig *config.MetaConfig) error {
	nodeName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, nodeGroupName, index)
	nodeConfig := metaConfig.PrepareTerraformNodeGroupConfig(nodeGroupName, index, cloudConfig)

	return logboek.LogProcess(fmt.Sprintf("🌿 ~ Bootstrap additional Node %s", nodeName), log.TaskOptions(), func() error {
		runner := terraform.NewRunner(providerName, layout, step).
			WithVariables(nodeConfig).
			WithStatePath("").
			WithAutoApprove(true)

		defer runner.Close()

		state, err := terraform.ApplyPipeline(runner, terraform.OnlyState)
		if err != nil {
			_ = runner.Destroy()
			return err
		}

		return SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, state["terraformState"])
	})
}

func BootstrapAdditionalMasterNode(kubeCl *client.KubernetesClient, index int, providerName, layout, cloudConfig string, metaConfig *config.MetaConfig) error {
	nodeName := fmt.Sprintf("%s-%s-%v", metaConfig.ClusterPrefix, masterNodeGroupName, index)
	nodeConfig := metaConfig.PrepareTerraformNodeGroupConfig(masterNodeGroupName, index, cloudConfig)

	return logboek.LogProcess(fmt.Sprintf("🌿 ~ Bootstrap additional Master Node %s", masterNodeGroupName), log.TaskOptions(), func() error {
		runner := terraform.NewRunner(providerName, layout, "master-node").
			WithVariables(nodeConfig).
			WithStatePath("").
			WithAutoApprove(true)

		defer runner.Close()

		state, err := terraform.ApplyPipeline(runner, terraform.GetMasterNodeResult)
		if err != nil {
			_ = runner.Destroy()
			return err
		}

		var devicePath string
		_ = json.Unmarshal(state["kubernetesDataDevicePath"], &devicePath)

		return SaveMasterNodeTerraformState(kubeCl, nodeName, state["terraformState"], []byte(devicePath))
	})
}

func RunConverge(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) error {
	if err := updateClusterState(kubeCl, metaConfig); err != nil {
		return err
	}

	nodesState, err := GetNodesStateFromCluster(kubeCl)
	if err != nil {
		return fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
	}

	var nodeGroupsWithStateInCluster []string
	for _, group := range metaConfig.GetStaticNodeGroups() {
		// Skip if node group terraform state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}
		if err := createPreviouslyNotExistentNodeGroup(kubeCl, metaConfig, group); err != nil {
			return err
		}
	}

	for _, nodeGroupName := range sortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		controller := NewConvergeController(kubeCl, metaConfig)
		if err := controller.Run(nodeGroupName, nodesState[nodeGroupName]); err != nil {
			return err
		}
	}
	return nil
}

func updateClusterState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) error {
	return logboek.LogProcess("✨ ~ Update cluster Terraform state", log.ConvergeOptions(), func() error {
		clusterState, err := GetClusterStateFromCluster(kubeCl)
		if err != nil {
			return fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
		}

		if clusterState == nil {
			return fmt.Errorf("kubernetes cluster has no state")
		}

		baseRunner := terraform.NewRunnerFromMetaConfig("base-infrastructure", metaConfig).
			WithVariables(metaConfig.MarshalConfig()).
			WithState(clusterState).
			WithAutoApprove(true)

		basePipelineResult, err := terraform.ApplyPipeline(baseRunner, terraform.OnlyState)
		if err != nil {
			return err
		}

		if err := SaveClusterTerraformState(kubeCl, basePipelineResult["terraformState"]); err != nil {
			return err
		}
		return nil
	})
}

func createPreviouslyNotExistentNodeGroup(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, group config.StaticNodeGroupSpec) error {
	return logboek.LogProcess(fmt.Sprintf("✨️ ~ Create new NodeGroup %s (replicas: %v)️", group.Name, group.Replicas), log.ConvergeOptions(), func() error {
		err := CreateNodeGroup(kubeCl, group.Name, metaConfig.MarshalNodeGroupConfig(group))
		if err != nil {
			return err
		}

		nodeCloudConfig, err := GetCloudConfig(kubeCl, group.Name)
		if err != nil {
			return err
		}

		for i := 0; i < group.Replicas; i++ {
			err = BootstrapAdditionalNode(kubeCl, i, metaConfig.ProviderName, metaConfig.Layout, "static-node", group.Name, nodeCloudConfig, metaConfig)
			if err != nil {
				return err
			}
		}

		if err := WaitForNodesBecomeReady(kubeCl, group.Name, group.Replicas); err != nil {
			return err
		}
		return nil
	})
}

type ConvergeController struct {
	client *client.KubernetesClient
	config *config.MetaConfig
}

type ConvergeNodeGroupGroupOptions struct {
	Name        string
	Step        string
	CloudConfig string
	Replicas    int
	State       map[string][]byte
}

func NewConvergeController(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) *ConvergeController {
	return &ConvergeController{client: kubeCl, config: metaConfig}
}

func (c *ConvergeController) Run(nodeGroupName string, nodeGroupState map[string][]byte) error {
	replicas := getReplicasByNodeGroupName(c.config, nodeGroupName)
	step := GetStepByNodeGroupName(nodeGroupName)

	return logboek.LogProcess(fmt.Sprintf("✨️ ~ Converge NodeGroup %s (replicas: %v)️", nodeGroupName, replicas), log.ConvergeOptions(), func() error {
		nodeCloudConfig, err := GetCloudConfig(c.client, nodeGroupName)
		if err != nil {
			return err
		}

		nodeGroup := ConvergeNodeGroupGroupOptions{
			Name:        nodeGroupName,
			Step:        step,
			Replicas:    replicas,
			CloudConfig: nodeCloudConfig,
			State:       nodeGroupState,
		}

		if replicas > len(nodeGroupState) {
			if err := c.addNewNodeGroup(&nodeGroup); err != nil {
				return err
			}
		}

		for name := range nodeGroupState {
			if err := c.updateNode(&nodeGroup, name); err != nil {
				return err
			}
		}

		if replicas < len(nodeGroupState) {
			if err := c.deleteRedundantNodes(&nodeGroup); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *ConvergeController) addNewNodeGroup(nodeGroup *ConvergeNodeGroupGroupOptions) error {
	return logboek.LogProcess(fmt.Sprintf("🌼 ~ Add new Nodes for NodeGroup %s (replicas: %v)", nodeGroup.Name, nodeGroup.Replicas), log.BoldOptions(), func() error {
		count := len(nodeGroup.State)
		index := 0

		for nodeGroup.Replicas > count {
			candidateName := fmt.Sprintf("%s-%s-%v", c.config.ClusterPrefix, nodeGroup.Name, index)
			if _, ok := nodeGroup.State[candidateName]; !ok {
				var err error
				if nodeGroup.Name == masterNodeGroupName {
					err = BootstrapAdditionalMasterNode(c.client, index, c.config.ProviderName, c.config.Layout, nodeGroup.CloudConfig, c.config)
				} else {
					err = BootstrapAdditionalNode(c.client, index, c.config.ProviderName, c.config.Layout, nodeGroup.Step, nodeGroup.Name, nodeGroup.CloudConfig, c.config)
				}
				if err != nil {
					return err
				}
				count++
			}
			index++
		}
		return WaitForNodesBecomeReady(c.client, nodeGroup.Name, nodeGroup.Replicas)
	})
}

func (c *ConvergeController) updateNode(nodeGroup *ConvergeNodeGroupGroupOptions, nodeName string) error {
	state := nodeGroup.State[nodeName]
	return logboek.LogProcess(fmt.Sprintf("🌻 ~ Update node %s", nodeName), log.TaskOptions(), func() error {
		index := getIndexFromNodeName(nodeName)
		if index == -1 {
			logboek.LogWarnF("can't extract index from terraform state secret, skip %s\n", nodeName)
			return nil
		}

		nodeRunner := terraform.NewRunnerFromMetaConfig(nodeGroup.Step, c.config).
			WithVariables(c.config.PrepareTerraformNodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
			WithState(state)

		nodeResult, err := terraform.ApplyPipeline(nodeRunner, terraform.OnlyState)
		if err != nil {
			return err
		}

		err = SaveNodeTerraformState(c.client, nodeName, nodeGroup.Name, nodeResult["terraformState"])
		if err != nil {
			return err
		}

		return WaitForSingleNodeBecomeReady(c.client, nodeName)
	})
}

func (c *ConvergeController) deleteRedundantNodes(nodeGroup *ConvergeNodeGroupGroupOptions) error {
	return logboek.LogProcess(fmt.Sprintf("🔥 ~ Delete redundant nodes for NodeGroup %s", nodeGroup.Name), log.TaskOptions(), func() error {
		deleteNodesNames := make(map[string][]byte)
		count := len(nodeGroup.State)

		for name, state := range nodeGroup.State {
			deleteNodesNames[name] = state
			delete(nodeGroup.State, name)
			count--

			if count == nodeGroup.Replicas {
				break
			}
		}

		for name, state := range deleteNodesNames {
			index := getIndexFromNodeName(name)
			if index == -1 {
				logboek.LogWarnF("can't extract index from terraform state secret, skip %s\n", name)
				continue
			}
			nodeRunner := terraform.NewRunnerFromMetaConfig(nodeGroup.Step, c.config).
				WithVariables(c.config.PrepareTerraformNodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
				WithState(state).
				WithAutoApprove(true)

			if err := terraform.DestroyPipeline(nodeRunner); err != nil {
				return err
			}

			nodeRunner.Close()
			err := DeleteTerraformState(c.client, fmt.Sprintf("d8-node-terraform-state-%s", name))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func getIndexFromNodeName(name string) int64 {
	index, err := strconv.ParseInt(name[strings.LastIndex(name, "-")+1:], 10, 64)
	if err != nil {
		logboek.LogWarnLn(err)
		return -1
	}
	return index
}

func getReplicasByNodeGroupName(metaConfig *config.MetaConfig, nodeGroupName string) int {
	replicas := 0
	if nodeGroupName != masterNodeGroupName {
		for _, group := range metaConfig.GetStaticNodeGroups() {
			if group.Name == nodeGroupName {
				replicas = group.Replicas
				break
			}
		}
	} else {
		replicas = metaConfig.MasterNodeGroupSpec.Replicas
	}
	return replicas
}

func GetStepByNodeGroupName(nodeGroupName string) string {
	step := "static-node"
	if nodeGroupName == masterNodeGroupName {
		step = "master-node"
	}
	return step
}

func sortNodeGroupsStateKeys(state map[string]map[string][]byte, sortedNodeGroupsFromConfig []string) []string {
	nodeGroupsFromConfigSet := make(map[string]struct{}, len(sortedNodeGroupsFromConfig))
	for _, key := range sortedNodeGroupsFromConfig {
		nodeGroupsFromConfigSet[key] = struct{}{}
	}

	sortedKeys := append([]string{masterNodeGroupName}, sortedNodeGroupsFromConfig...)

	for key := range state {
		if key == masterNodeGroupName {
			continue
		}

		if _, ok := nodeGroupsFromConfigSet[key]; !ok {
			sortedKeys = append(sortedKeys, key)
		}
	}

	return sortedKeys
}
