package converge

import "github.com/deckhouse/deckhouse/dhctl/pkg/config"

func getReplicasByNodeGroupName(metaConfig *config.MetaConfig, nodeGroupName string) int {
	replicas := 0
	if nodeGroupName != masterNodeGroupName {
		for _, group := range metaConfig.GetTerraNodeGroups() {
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

func getStepByNodeGroupName(nodeGroupName string) string {
	step := "static-node"
	if nodeGroupName == masterNodeGroupName {
		step = "master-node"
	}
	return step
}

func sortNodeGroupsStateKeys(state map[string]NodeGroupTerraformState, sortedNodeGroupsFromConfig []string) []string {
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
