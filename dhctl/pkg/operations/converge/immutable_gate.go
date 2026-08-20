// Copyright 2026 Flant JSC
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
	gocontext "context"
	"fmt"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// convergedNodeGroupNames lists the groups the node phases walk: the master group, the
// CloudPermanent groups of the config, and the groups that still hold state in the
// cluster after being dropped from the config — converge deletes their nodes.
func convergedNodeGroupNames(metaConfig *config.MetaConfig, nodesState map[string]state.NodeGroupInfrastructureState) []string {
	terraNodeGroups := metaConfig.GetTerraNodeGroups()

	names := make([]string, 0, 1+len(terraNodeGroups)+len(nodesState))
	names = append(names, global.MasterNodeGroupName)

	for _, group := range terraNodeGroups {
		names = append(names, group.Name)
	}

	for name := range nodesState {
		names = append(names, name)
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// refuseImmutableNodeGroups stops the node phases while a group they walk is immutable.
// Such a node is driven by its agent from a NodeConfig, and converge would hand a newly
// created VM the group-wide bashible cloud-init instead: the VM boots and never joins.
// The refusal is lifted once converge learns to render the join payload.
//
// Fail-closed: an unreadable NodeGroup list refuses the phase too. Groups converge never
// walks (CloudEphemeral and the rest) are left alone, so a bashible cluster keeps
// converging while its ephemeral nodes migrate to an immutable system.
func refuseImmutableNodeGroups(
	ctx gocontext.Context,
	kubeGetter kubernetes.KubeClientProviderWithCtx,
	convergedGroups []string,
) error {
	kubeCl, err := kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		return fmt.Errorf("get kube client to read NodeGroups: %w", err)
	}

	nodeGroups, err := entity.GetNodeGroups(ctx, kubeCl)
	if err != nil {
		return fmt.Errorf("read NodeGroups to tell mutable nodes from immutable ones: %w", err)
	}

	var immutableGroups []string
	for _, ng := range nodeGroups {
		if !slices.Contains(convergedGroups, ng.GetName()) {
			continue
		}

		if immutable.NodeGroupIsImmutable(&ng) {
			immutableGroups = append(immutableGroups, ng.GetName())
		}
	}

	if len(immutableGroups) == 0 {
		return nil
	}

	return fmt.Errorf(
		"converge of nodes is not supported yet for immutable NodeGroups: %s. "+
			"Nodes of such a group are reconciled by the on-node agent from a NodeConfig, while converge would give a "+
			"newly created VM the bashible cloud-init of the group — the VM would boot and never join the cluster. "+
			"The base infrastructure phase converges on its own and is not affected",
		strings.Join(immutableGroups, ", "),
	)
}
