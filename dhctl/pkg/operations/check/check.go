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

package check

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/mitchellh/copystructure"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
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
	Status             string                                               `json:"status,omitempty"`
	DestructiveChanges *infrastructure.BaseInfrastructureDestructiveChanges `json:"destructive_changes,omitempty"`
}

type NodeCheckResult struct {
	Group              string                   `json:"group,omitempty"`
	Name               string                   `json:"name,omitempty"`
	Status             string                   `json:"status,omitempty"`
	DestructiveChanges *plan.DestructiveChanges `json:"destructive_changes,omitempty"`
}

type NodeGroupCheckResult struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type Statistics struct {
	Node               []NodeCheckResult                     `json:"nodes,omitempty"`
	NodeTemplates      []NodeGroupCheckResult                `json:"node_templates,omitempty"`
	Cluster            ClusterCheckResult                    `json:"cluster,omitempty"`
	InfrastructurePlan []plan.Plan                           `json:"terraform_plan,omitempty"`
	TerraformVersion   *infrastructurestate.TerraformVersion `json:"terraform_version,omitempty"`
}

type NodeGroupOptions struct {
	Name            string
	LayoutStep      infrastructure.Step
	CloudConfig     string
	DesiredReplicas int
	State           map[string][]byte
}

// Format data according to the specified format ("json"|"yaml") and
// hides raw infrastructure plan and destructive changes from result
func (s Statistics) Format(outputFormat string) ([]byte, error) {
	copied, err := copystructure.Copy(s)
	if err != nil {
		return nil, fmt.Errorf("unable to copy check statistics")
	}

	printableStatistics := copied.(Statistics)
	printableStatistics.InfrastructurePlan = nil
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

type ClusterStateCheckResult struct {
	Change             int
	Plan               plan.Plan
	DestructiveChanges *infrastructure.BaseInfrastructureDestructiveChanges
	IsTerraformState   bool
}

func checkClusterState(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, infrastructureContext *infrastructure.Context, opts CheckStateOptions) (*ClusterStateCheckResult, error) {
	var clusterState []byte
	var err error
	// NOTE: Cluster state loaded from target kubernetes cluster in default dhctl-converge.
	// NOTE: In the commander mode cluster state should exist in the local state cache.
	if !opts.CommanderMode {
		clusterState, err = infrastructurestate.GetClusterStateFromCluster(ctx, kubeCl)
		if err != nil {
			return nil, fmt.Errorf("infrastructure cluster state in Kubernetes cluster not found: %w", err)
		}
		if clusterState == nil {
			return nil, fmt.Errorf("kubernetes cluster has no state")
		}
	}

	var stateSavers []infrastructure.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, infrastructurestate.NewClusterStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl)))
	}

	baseRunner, err := infrastructureContext.GetCheckBaseInfraRunner(ctx, metaConfig, infrastructure.BaseInfraRunnerOptions{
		CommanderMode: opts.CommanderMode,

		StateCache:                       opts.StateCache,
		ClusterState:                     clusterState,
		AdditionalStateSaverDestinations: stateSavers,
	})
	if err != nil {
		return nil, err
	}

	change, pl, destructiveChanges, err := infrastructure.CheckBaseInfrastructurePipeline(ctx, baseRunner, "Kubernetes cluster")
	if err != nil {
		return nil, err
	}

	st, err := baseRunner.GetState()
	if err != nil {
		return nil, err
	}

	isTerraformState, err := infrastructurestate.IsTerraformState(st)
	if err != nil {
		return nil, err
	}

	return &ClusterStateCheckResult{
		Change:             change,
		Plan:               pl,
		DestructiveChanges: destructiveChanges,
		IsTerraformState:   isTerraformState,
	}, nil
}

func checkAbandonedNodeState(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, nodeGroup *NodeGroupOptions, nodeGroupState *state.NodeGroupInfrastructureState, nodeName string, infrastructureContext *infrastructure.Context, opts CheckStateOptions) (int, plan.Plan, *plan.DestructiveChanges, error) {
	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		return plan.HasNoChanges, nil, nil, fmt.Errorf("can't extract index from infrastructure state secret (%v), skip %s", err, nodeName)
	}

	cfg := metaConfig
	if nodeGroupState.Settings != nil {
		nodeGroupsSettings, err := json.Marshal([]json.RawMessage{nodeGroupState.Settings})
		if err != nil {
			log.ErrorLn(err)
		} else {
			// we use dummy preparator because metaConfig was prepared early
			cfg, err = metaConfig.DeepCopy().Prepare(ctx, config.DummyPreparatorProvider())
			if err != nil {
				return plan.HasNoChanges, nil, nil, fmt.Errorf("unable to prepare copied config: %v", err)
			}
			cfg.ProviderClusterConfig["nodeGroups"] = nodeGroupsSettings
		}
	}

	pipelineForMaster := nodeGroup.LayoutStep == infrastructure.MasterNodeStep
	nodeGroupName := nodeGroup.Name
	if pipelineForMaster {
		nodeGroupName = global.MasterNodeGroupName
	}

	var stateSavers []infrastructure.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, infrastructurestate.NewNodeStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl), nodeName, nodeGroupName, nil))
	}
	nodeRunner, err := infrastructureContext.GetCheckNodeDeleteRunner(ctx, cfg, infrastructure.NodeDeleteRunnerOptions{
		NodeName:                         nodeName,
		NodeGroupName:                    nodeGroup.Name,
		LayoutStep:                       nodeGroup.LayoutStep,
		NodeIndex:                        nodeIndex,
		NodeState:                        nodeGroup.State[nodeName],
		NodeCloudConfig:                  nodeGroup.CloudConfig,
		CommanderMode:                    opts.CommanderMode,
		StateCache:                       opts.StateCache,
		AdditionalStateSaverDestinations: stateSavers,
	})
	if err != nil {
		return plan.HasNoChanges, nil, nil, err
	}

	return infrastructure.CheckPipeline(ctx, nodeRunner, nodeName, true, false)
}

type NodeStateCheckResult struct {
	Change             int
	Plan               plan.Plan
	DestructiveChanges *plan.DestructiveChanges
	IsTerraformState   bool
}

func checkNodeState(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, nodeGroup *NodeGroupOptions, nodeName string, infrastructureContext *infrastructure.Context, opts CheckStateOptions, noout bool) (*NodeStateCheckResult, error) {
	nodeIndex, err := config.GetIndexFromNodeName(nodeName)
	if err != nil {
		return nil, fmt.Errorf("can't extract index from infrastructure state secret (%v), skip %s", err, nodeName)
	}

	pipelineForMaster := nodeGroup.LayoutStep == infrastructure.MasterNodeStep

	nodeGroupName := nodeGroup.Name
	var nodeGroupSettingsFromConfig []byte
	if pipelineForMaster {
		nodeGroupName = global.MasterNodeGroupName
	} else {
		// Node group settings are only for the static node.
		nodeGroupSettingsFromConfig = metaConfig.FindTerraNodeGroup(nodeGroup.Name)
	}

	var stateSavers []infrastructure.SaverDestination
	if opts.CommanderMode {
		stateSavers = append(stateSavers, infrastructurestate.NewNodeStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl), nodeName, nodeGroupName, nodeGroupSettingsFromConfig))
	}

	nodeRunner, err := infrastructureContext.GetCheckNodeRunner(ctx, metaConfig, infrastructure.NodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupName:   nodeGroup.Name,
		NodeGroupStep:   nodeGroup.LayoutStep,
		NodeIndex:       nodeIndex,
		NodeState:       nodeGroup.State[nodeName],
		NodeCloudConfig: nodeGroup.CloudConfig,

		CommanderMode:                    opts.CommanderMode,
		StateCache:                       opts.StateCache,
		AdditionalStateSaverDestinations: stateSavers,
	})
	if err != nil {
		return nil, err
	}

	change, pl, destructiveChanges, err := infrastructure.CheckPipeline(ctx, nodeRunner, nodeName, false, noout)
	if err != nil {
		return nil, err
	}

	st, err := nodeRunner.GetState()
	if err != nil {
		return nil, err
	}

	isTerraformState, err := infrastructurestate.IsTerraformState(st)
	if err != nil {
		return nil, err
	}

	return &NodeStateCheckResult{
		Change:             change,
		Plan:               pl,
		DestructiveChanges: destructiveChanges,
		IsTerraformState:   isTerraformState,
	}, nil
}

type CheckStateOptions struct {
	CommanderMode bool
	StateCache    dhctlstate.Cache
}

func CheckState(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, infrastructureContext *infrastructure.Context, opts CheckStateOptions, noout bool) (*Statistics, bool, error) {
	statistics := Statistics{
		Node:          make([]NodeCheckResult, 0),
		NodeTemplates: make([]NodeGroupCheckResult, 0),
		Cluster:       ClusterCheckResult{Status: OKStatus},
	}

	var allErrs *multierror.Error

	// clusterChanged, plan, destructiveChanges,
	baseRes, err := checkClusterState(ctx, kubeCl, metaConfig, infrastructureContext, opts)
	switch {
	case err != nil:
		statistics.Cluster.Status = ErrorStatus
		allErrs = multierror.Append(allErrs, err)
	case baseRes.Change == plan.HasChanges:
		statistics.Cluster.Status = ChangedStatus
	case baseRes.Change == plan.HasDestructiveChanges:
		statistics.Cluster.Status = DestructiveStatus
		statistics.Cluster.DestructiveChanges = baseRes.DestructiveChanges
	}

	hasTerraformState := false

	if baseRes != nil {
		if baseRes.Plan != nil {
			statistics.InfrastructurePlan = append(statistics.InfrastructurePlan, baseRes.Plan)
		}

		hasTerraformState = baseRes.IsTerraformState
	}

	log.DebugF("Base infrastructure has terraform state %v\n", hasTerraformState)

	// NOTE: Nodes state loaded from target kubernetes cluster in default dhctl-converge.
	// NOTE: In the commander mode nodes state should exist in the local state cache.
	var nodesState map[string]state.NodeGroupInfrastructureState
	if opts.CommanderMode {
		nodesState, err = LoadNodesStateForCommanderMode(ctx, opts.StateCache, metaConfig, kubeCl)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to load nodes state: %w", err))
		}
	} else {
		nodesState, err = infrastructurestate.GetNodesStateFromCluster(ctx, kubeCl)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("infrastructure cluster state in Kubernetes cluster not found: %w", err))
		}
	}

	nodeTemplates, err := entity.GetNodeGroupTemplates(ctx, kubeCl)
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

		// Skip if node group infrastructure state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}

		// track missed
		for _, nodeName := range expectedNodeNames(metaConfig, group.Name, group.Replicas) {
			result := getStatusForMissedNode(ctx, kubeCl, nodeName, group.Name, &allErrs)
			statistics.Node = append(statistics.Node, result)
		}
	}

	for _, nodeGroupName := range utils.SortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		nodeGroupState := nodesState[nodeGroupName]
		replicas := metaConfig.GetReplicasByNodeGroupName(nodeGroupName)
		layoutStep := infrastructure.GetStepByNodeGroupName(nodeGroupName)

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
				result := getStatusForMissedNode(ctx, kubeCl, nodeName, nodeGroupName, &allErrs)
				statistics.Node = append(statistics.Node, result)
			}
		} else if replicas < len(nodeGroupState.State) {
			sortedNodeNames, err := sortNodesByIndex(nodeGroupState.State)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
				continue
			}

			nodeGroup := NodeGroupOptions{
				Name:            nodeGroupName,
				LayoutStep:      layoutStep,
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

				_, infrastructurePlan, destructiveChanges, err := checkAbandonedNodeState(ctx, kubeCl, metaConfig, &nodeGroup, &nodeGroupState, nodeName, infrastructureContext, opts)
				if err != nil {
					checkResult.Status = ErrorStatus
					allErrs = multierror.Append(allErrs, fmt.Errorf("node %s: %v", nodeName, err))
				} else {
					checkResult.Status = AbandonedStatus
					checkResult.DestructiveChanges = destructiveChanges
				}

				statistics.Node = append(statistics.Node, checkResult)
				if infrastructurePlan != nil {
					statistics.InfrastructurePlan = append(statistics.InfrastructurePlan, infrastructurePlan)
				}

				sortedNodeNames = sortedNodeNames[:lastIndex]
				delete(nodeGroupState.State, nodeName)
				excessiveQuantity--
			}
		}

		nodeGroup := NodeGroupOptions{
			Name:            nodeGroupName,
			LayoutStep:      layoutStep,
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
			// changed, infrastructurePlan, destructiveChanges,
			nodeRes, err := checkNodeState(ctx, kubeCl, metaConfig, &nodeGroup, name, infrastructureContext, opts, noout)
			switch {
			case err != nil:
				checkResult.Status = ErrorStatus
				allErrs = multierror.Append(allErrs, fmt.Errorf("node %s: %v", name, err))
			case nodeRes.Change == plan.HasChanges:
				checkResult.Status = ChangedStatus
			case nodeRes.Change == plan.HasDestructiveChanges:
				checkResult.Status = DestructiveStatus
				checkResult.DestructiveChanges = nodeRes.DestructiveChanges
			}

			statistics.Node = append(statistics.Node, checkResult)
			if nodeRes != nil {
				if nodeRes.Plan != nil {
					statistics.InfrastructurePlan = append(statistics.InfrastructurePlan, nodeRes.Plan)
				}

				log.DebugF("Node %s has terraform state: %v\n", name, nodeRes.IsTerraformState)

				if nodeRes.IsTerraformState && !hasTerraformState {
					hasTerraformState = nodeRes.IsTerraformState
					log.DebugF("Has terraform state after node %s: %v\n", name, hasTerraformState)
				}
			}
		}
	}

	return &statistics, hasTerraformState, allErrs.ErrorOrNil()
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

func getStatusForMissedNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName, nodeGroupName string, allErrs **multierror.Error) NodeCheckResult {
	status := AbsentStatus

	exists, err := entity.IsNodeExistsInCluster(ctx, kubeCl, nodeName, log.GetDefaultLogger())
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
