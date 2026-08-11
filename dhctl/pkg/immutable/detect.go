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

package immutable

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
)

// systemTypeImmutable mirrors the node-manager API. node-controller is a
// separate Go module, so the constant is repeated here instead of imported.
const systemTypeImmutable = "Immutable"

// IsImmutableMaster reports whether the master NodeGroup asks for an immutable
// system. The resources section is already parsed into CloudProviderVars by
// config.Prepare, which rejects a document it cannot read.
func IsImmutableMaster(_ context.Context, metaConfig *config.MetaConfig) bool {
	if metaConfig == nil || metaConfig.CloudProviderVars == nil {
		return false
	}

	master := metaConfig.CloudProviderVars.NodeGroups[global.MasterNodeGroupName]
	systemType, _, _ := unstructured.NestedString(master, "spec", "systemType")

	return systemType == systemTypeImmutable
}

// ValidateClusterType rejects an immutable master outside a cloud cluster. The
// node runs no sshd, so dhctl only ever reaches it at the address the BaseInfra
// phase reports — and that phase creates nothing outside a cloud cluster, which
// leaves the bootstrap with a node it has no way to talk to. Pure.
func ValidateClusterType(_ context.Context, metaConfig *config.MetaConfig) error {
	if metaConfig.ClusterType == config.CloudClusterType {
		return nil
	}

	return fmt.Errorf(
		"the master NodeGroup asks for systemType %q, which is supported in a %q cluster only, and this one is %q: "+
			"such a node answers no sshd and dhctl reaches its API server at the address the cloud infrastructure reports, "+
			"which is never reported here. Drop systemType from the master NodeGroup, or bootstrap a cloud cluster",
		systemTypeImmutable, config.CloudClusterType, metaConfig.ClusterType,
	)
}
