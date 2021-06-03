package converge

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func BootstrapAdditionalNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, step, nodeGroupName, cloudConfig string, intermediateStateSave bool) error {
	nodeName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, index)

	nodeExists, err := IsNodeExistsInCluster(kubeCl, nodeName)
	if err != nil {
		return err
	} else if nodeExists {
		return fmt.Errorf("node with name %s exists in cluster", nodeName)
	}

	nodeConfig := cfg.NodeGroupConfig(nodeGroupName, index, cloudConfig)
	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)

	runner := terraform.NewRunnerFromConfig(cfg, step).
		WithVariables(nodeConfig).
		WithName(nodeName).
		WithAutoApprove(true)
	tomb.RegisterOnShutdown(nodeName, runner.Stop)

	if intermediateStateSave {
		runner.WithIntermediateStateSaver(NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettings))
	}

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return ErrConvergeInterrupted
	}

	err = SaveNodeTerraformState(kubeCl, nodeName, nodeGroupName, outputs.TerraformState, nodeGroupSettings)
	if err != nil {
		return err
	}

	return nil
}

func BootstrapAdditionalMasterNode(kubeCl *client.KubernetesClient, cfg *config.MetaConfig, index int, cloudConfig string, intermediateStateSave bool) (*terraform.PipelineOutputs, error) {
	nodeName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, masterNodeGroupName, index)

	nodeExists, existsErr := IsNodeExistsInCluster(kubeCl, nodeName)
	if existsErr != nil {
		return nil, existsErr
	} else if nodeExists {
		return nil, fmt.Errorf("node with name %s exists in cluster", nodeName)
	}

	nodeConfig := cfg.NodeGroupConfig(masterNodeGroupName, index, cloudConfig)

	runner := terraform.NewRunnerFromConfig(cfg, "master-node").
		WithVariables(nodeConfig).
		WithName(nodeName).
		WithAutoApprove(true)
	tomb.RegisterOnShutdown(nodeName, runner.Stop)

	// Node group settings are not required for master node secret.
	if intermediateStateSave {
		runner.WithIntermediateStateSaver(NewNodeStateSaver(kubeCl, nodeName, masterNodeGroupName, nil))
	}

	outputs, err := terraform.ApplyPipeline(runner, nodeName, terraform.GetMasterNodeResult)
	if err != nil {
		return nil, err
	}

	if tomb.IsInterrupted() {
		return nil, ErrConvergeInterrupted
	}

	err = SaveMasterNodeTerraformState(kubeCl, nodeName, outputs.TerraformState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return outputs, err
	}

	return outputs, err
}
