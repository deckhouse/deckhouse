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

package converge

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/mitchellh/copystructure"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	state_terraform "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

const (
	OKStatus          = "ok"
	ChangedStatus     = "changed"
	DestructiveStatus = "destructively_changed"
	AbandonedStatus   = "abandoned"
	AbsentStatus      = "absent"
	ErrorStatus       = "error"
)

type ClusterCheckResult struct {
	Status             string                                          `json:"status,omitempty"`
	DestructiveChanges *terraform.BaseInfrastructureDestructiveChanges `json:"destructive_changes,omitempty"`
}

type NodeCheckResult struct {
	Group              string                            `json:"group,omitempty"`
	Name               string                            `json:"name,omitempty"`
	Status             string                            `json:"status,omitempty"`
	DestructiveChanges *terraform.PlanDestructiveChanges `json:"destructive_changes,omitempty"`
}

type NodeGroupCheckResult struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type Statistics struct {
	Node          []NodeCheckResult         `json:"nodes,omitempty"`
	NodeTemplates []NodeGroupCheckResult    `json:"node_templates,omitempty"`
	Cluster       ClusterCheckResult        `json:"cluster,omitempty"`
	TerraformPlan []terraform.TerraformPlan `json:"terraform_plan,omitempty"`
}

// Format data according to the specified format ("json"|"yaml") and
// hides raw terraform plan and destructive changes from result
func (s Statistics) Format(outputFormat string) ([]byte, error) {
	copied, err := copystructure.Copy(s)
	if err != nil {
		return nil, fmt.Errorf("unable to copy check statistics")
	}

	printableStatistics := copied.(Statistics)
	printableStatistics.TerraformPlan = nil
	printableStatistics.Cluster.DestructiveChanges = nil
	for i := range printableStatistics.Node {
		printableStatistics.Node[i].DestructiveChanges = nil
	}

	var data []byte
	switch outputFormat {
	case "yaml":
		data, err = yaml.Marshal(printableStatistics)
		if err != nil {
			return nil, err
		}
	case "json":
		data, err = json.Marshal(printableStatistics)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown output format %s", outputFormat)
	}

	return data, nil
}

func checkClusterState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraformContext *terraform.TerraformContext, opts CheckStateOptions) (int, terraform.TerraformPlan, *terraform.BaseInfrastructureDestructiveChanges, error) {
	var clusterState []byte
	var err error
	// NOTE: Cluster state loaded from target kubernetes cluster in default dhctl-converge.
	// NOTE: In the commander mode cluster state should exist in the local state cache.
	if !opts.CommanderMode {
		clusterState, err = state_terraform.GetClusterStateFromCluster(kubeCl)
		if err != nil {
			return terraform.PlanHasNoChanges, nil, nil, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
		}
		if clusterState == nil {
			return terraform.PlanHasNoChanges, nil, nil, fmt.Errorf("kubernetes cluster has no state")
		}
	}

	var stateSavers []terraform.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, NewClusterStateSaver(kubeCl))
	}

	baseRunner := terraformContext.GetCheckBaseInfraRunner(metaConfig, terraform.BaseInfraRunnerOptions{
		AutoDismissDestructive:           false,
		AutoApprove:                      true,
		CommanderMode:                    opts.CommanderMode,
		StateCache:                       opts.StateCache,
		ClusterState:                     clusterState,
		AdditionalStateSaverDestinations: stateSavers,
	})

	return terraform.CheckBaseInfrastructurePipeline(baseRunner, "Kubernetes cluster")
}

func checkAbandonedNodeState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, nodeGroup *NodeGroupGroupOptions, nodeGroupState *state.NodeGroupTerraformState, nodeName string, terraformContext *terraform.TerraformContext, opts CheckStateOptions) (int, terraform.TerraformPlan, *terraform.PlanDestructiveChanges, error) {
	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		return terraform.PlanHasNoChanges, nil, nil, fmt.Errorf("can't extract index from terraform state secret (%v), skip %s", err, nodeName)
	}

	cfg := metaConfig
	if nodeGroupState.Settings != nil {
		nodeGroupsSettings, err := json.Marshal([]json.RawMessage{nodeGroupState.Settings})
		if err != nil {
			log.ErrorLn(err)
		} else {
			cfg, err = metaConfig.DeepCopy().Prepare()
			if err != nil {
				return terraform.PlanHasNoChanges, nil, nil, fmt.Errorf("unable to prepare copied config: %v", err)
			}
			cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
		}
	}

	pipelineForMaster := nodeGroup.Step == "master-node"
	nodeGroupName := nodeGroup.Name
	if pipelineForMaster {
		nodeGroupName = MasterNodeGroupName
	}

	var stateSavers []terraform.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nil))
	}
	nodeRunner := terraformContext.GetCheckNodeDeleteRunner(cfg, terraform.NodeDeleteRunnerOptions{
		AutoDismissDestructive:           false,
		AutoApprove:                      true,
		NodeName:                         nodeName,
		NodeGroupName:                    nodeGroup.Name,
		NodeGroupStep:                    nodeGroup.Step,
		NodeIndex:                        nodeIndex,
		NodeState:                        nodeGroup.State[nodeName],
		NodeCloudConfig:                  nodeGroup.CloudConfig,
		CommanderMode:                    opts.CommanderMode,
		StateCache:                       opts.StateCache,
		AdditionalStateSaverDestinations: stateSavers,
	})

	return terraform.CheckPipeline(nodeRunner, nodeName, terraform.PlanOptions{Destroy: true})
}

func checkNodeState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, nodeGroup *NodeGroupGroupOptions, nodeName string, terraformContext *terraform.TerraformContext, opts CheckStateOptions) (int, terraform.TerraformPlan, *terraform.PlanDestructiveChanges, error) {
	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		return terraform.PlanHasNoChanges, nil, nil, fmt.Errorf("can't extract index from terraform state secret (%v), skip %s", err, nodeName)
	}

	pipelineForMaster := nodeGroup.Step == "master-node"

	nodeGroupName := nodeGroup.Name
	var nodeGroupSettingsFromConfig []byte
	if pipelineForMaster {
		nodeGroupName = MasterNodeGroupName
	} else {
		// Node group settings are only for the static node.
		nodeGroupSettingsFromConfig = metaConfig.FindTerraNodeGroup(nodeGroup.Name)
	}

	var stateSavers []terraform.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, NewNodeStateSaver(kubeCl, nodeName, nodeGroupName, nodeGroupSettingsFromConfig))
	}

	nodeRunner := terraformContext.GetCheckNodeRunner(metaConfig, terraform.NodeRunnerOptions{
		AutoDismissDestructive: false,
		AutoApprove:            true,

		NodeName:        nodeName,
		NodeGroupName:   nodeGroup.Name,
		NodeGroupStep:   nodeGroup.Step,
		NodeIndex:       nodeIndex,
		NodeState:       nodeGroup.State[nodeName],
		NodeCloudConfig: nodeGroup.CloudConfig,

		CommanderMode:                    opts.CommanderMode,
		StateCache:                       opts.StateCache,
		AdditionalStateSaverDestinations: stateSavers,
	})

	return terraform.CheckPipeline(nodeRunner, nodeName, terraform.PlanOptions{})
}

type CheckStateOptions struct {
	CommanderMode bool
	StateCache    dhctlstate.Cache
}

func CheckState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraformContext *terraform.TerraformContext, opts CheckStateOptions) (*Statistics, error) {
	statistics := Statistics{
		Node:          make([]NodeCheckResult, 0),
		NodeTemplates: make([]NodeGroupCheckResult, 0),
		Cluster:       ClusterCheckResult{Status: OKStatus},
	}

	var allErrs *multierror.Error

	clusterChanged, terraformPlan, destructiveChanges, err := checkClusterState(kubeCl, metaConfig, terraformContext, opts)
	switch {
	case err != nil:
		statistics.Cluster.Status = ErrorStatus
		allErrs = multierror.Append(allErrs, err)
	case clusterChanged == terraform.PlanHasChanges:
		statistics.Cluster.Status = ChangedStatus
	case clusterChanged == terraform.PlanHasDestructiveChanges:
		statistics.Cluster.Status = DestructiveStatus
		statistics.Cluster.DestructiveChanges = destructiveChanges
	}
	if terraformPlan != nil {
		statistics.TerraformPlan = append(statistics.TerraformPlan, terraformPlan)
	}

	// NOTE: Nodes state loaded from target kubernetes cluster in default dhctl-converge.
	// NOTE: In the commander mode nodes state should exist in the local state cache.
	var nodesState map[string]state.NodeGroupTerraformState
	if opts.CommanderMode {
		nodesState, err = LoadNodesStateForCommanderMode(opts.StateCache, metaConfig, kubeCl)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to load nodes state: %w", err))
		}
	} else {
		nodesState, err = state_terraform.GetNodesStateFromCluster(kubeCl)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err))
		}
	}

	nodeTemplates, err := GetNodeGroupTemplates(kubeCl)
	if err != nil {
		allErrs = multierror.Append(allErrs, fmt.Errorf("node goups in Kubernetes cluster not found: %w", err))
	}

	// We have no nodeTemplate settings for master nodes
	statistics.NodeTemplates = append(statistics.NodeTemplates, NodeGroupCheckResult{Name: "master", Status: OKStatus})

	var nodeGroupsWithStateInCluster []string
	for _, group := range metaConfig.GetTerraNodeGroups() {
		templateStatus := OKStatus

		if template, ok := nodeTemplates[group.Name]; ok {
			if !reflect.DeepEqual(template, group.NodeTemplate) {
				templateStatus = ChangedStatus
			}
		} else {
			templateStatus = AbsentStatus
		}
		statistics.NodeTemplates = append(statistics.NodeTemplates, NodeGroupCheckResult{Name: group.Name, Status: templateStatus})

		// Skip if node group terraform state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}

		// track missed
		for _, nodeName := range expectedNodeNames(metaConfig, group.Name, group.Replicas) {
			result := getStatusForMissedNode(kubeCl, nodeName, group.Name, &allErrs)
			statistics.Node = append(statistics.Node, result)
		}
	}

	for _, nodeGroupName := range sortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		nodeGroupState := nodesState[nodeGroupName]
		replicas := getReplicasByNodeGroupName(metaConfig, nodeGroupName)
		step := getStepByNodeGroupName(nodeGroupName)

		if replicas > len(nodeGroupState.State) {
			insufficientQuantity := len(nodeGroupState.State)
			var missedNodes []string

			for _, nodeName := range expectedNodeNames(metaConfig, nodeGroupName, replicas) {
				if _, ok := nodeGroupState.State[nodeName]; ok {
					insufficientQuantity--
				} else {
					missedNodes = append(missedNodes, nodeName)
				}
			}

			// this can happen because nodes in cluster are not normilized
			// for example, there can be three nodes in a cluster: node-1, node-3 and node-9
			if insufficientQuantity > 0 {
				missedNodes = missedNodes[:insufficientQuantity]
			}

			for _, nodeName := range missedNodes {
				result := getStatusForMissedNode(kubeCl, nodeName, nodeGroupName, &allErrs)
				statistics.Node = append(statistics.Node, result)
			}
		} else if replicas < len(nodeGroupState.State) {
			sortedNodeNames, err := sortNodesByIndex(nodeGroupState.State)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
				continue
			}

			nodeGroup := NodeGroupGroupOptions{
				Name:            nodeGroupName,
				Step:            step,
				DesiredReplicas: replicas,
				State:           nodeGroupState.State,
			}

			excessiveQuantity := len(nodeGroupState.State) - replicas
			for excessiveQuantity > 0 {
				lastIndex := len(sortedNodeNames) - 1
				nodeName := sortedNodeNames[lastIndex]

				checkResult := NodeCheckResult{
					Group: nodeGroupName,
					Name:  nodeName,
				}

				_, terraformPlan, destructiveChanges, err := checkAbandonedNodeState(kubeCl, metaConfig, &nodeGroup, &nodeGroupState, nodeName, terraformContext, opts)
				if err != nil {
					checkResult.Status = ErrorStatus
					allErrs = multierror.Append(allErrs, fmt.Errorf("node %s: %v", nodeName, err))
				} else {
					checkResult.Status = AbandonedStatus
					checkResult.DestructiveChanges = destructiveChanges
				}

				statistics.Node = append(statistics.Node, checkResult)
				if terraformPlan != nil {
					statistics.TerraformPlan = append(statistics.TerraformPlan, terraformPlan)
				}

				sortedNodeNames = sortedNodeNames[:lastIndex]
				delete(nodeGroupState.State, nodeName)
				excessiveQuantity--
			}
		}

		nodeGroup := NodeGroupGroupOptions{
			Name:            nodeGroupName,
			Step:            step,
			DesiredReplicas: replicas,
			State:           nodeGroupState.State,
		}

		for name := range nodeGroupState.State {
			// track changed and ok
			checkResult := NodeCheckResult{
				Group:  nodeGroupName,
				Name:   name,
				Status: OKStatus,
			}
			changed, terraformPlan, destructiveChanges, err := checkNodeState(kubeCl, metaConfig, &nodeGroup, name, terraformContext, opts)
			switch {
			case err != nil:
				checkResult.Status = ErrorStatus
				allErrs = multierror.Append(allErrs, fmt.Errorf("node %s: %v", name, err))
			case changed == terraform.PlanHasChanges:
				checkResult.Status = ChangedStatus
			case changed == terraform.PlanHasDestructiveChanges:
				checkResult.Status = DestructiveStatus
				checkResult.DestructiveChanges = destructiveChanges
			}

			statistics.Node = append(statistics.Node, checkResult)
			if terraformPlan != nil {
				statistics.TerraformPlan = append(statistics.TerraformPlan, terraformPlan)
			}
		}
	}

	return &statistics, allErrs.ErrorOrNil()
}

func expectedNodeNames(cfg *config.MetaConfig, nodeGroupName string, replicas int) []string {
	names := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		names = append(names, fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, i))
	}

	return names
}

func sortNodesByIndex(nodesState map[string][]byte) ([]string, error) {
	nameByIndex := make(map[int]string)
	order := make([]int, 0, len(nodesState))

	for nodeName := range nodesState {
		nodeIndex, err := config.GetIndexFromNodeName(nodeName)
		if err != nil {
			return nil, fmt.Errorf("cannot get index from node name %s: %v", nodeName, err)
		}

		order = append(order, nodeIndex)
		nameByIndex[nodeIndex] = nodeName
	}

	sort.Ints(order)
	names := make([]string, 0, len(nameByIndex))

	for _, i := range order {
		names = append(names, nameByIndex[i])
	}

	return names, nil
}

func getStatusForMissedNode(kubeCl *client.KubernetesClient, nodeName, nodeGroupName string, allErrs **multierror.Error) NodeCheckResult {
	status := AbsentStatus

	exists, err := IsNodeExistsInCluster(kubeCl, nodeName)
	if err != nil {
		*allErrs = multierror.Append(*allErrs, err)
		status = ErrorStatus
	}

	if exists {
		status = ErrorStatus
	}

	return NodeCheckResult{
		Group:  nodeGroupName,
		Name:   nodeName,
		Status: status,
	}
}
