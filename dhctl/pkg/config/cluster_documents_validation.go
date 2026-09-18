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

package config

import (
	"context"
	"fmt"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// validateClusterDocuments rejects the combinations of documents that do not describe a cluster
// anyone can build, and warns about the one that builds a cluster nobody wants.
func validateClusterDocuments(ctx context.Context, m *MetaConfig) error {
	if err := validateClusterTypeAgainstDocuments(m); err != nil {
		return err
	}

	warnAboutMasterReplicaParity(ctx, m)
	return nil
}

// validateClusterTypeAgainstDocuments refuses a clusterType that disagrees with the documents
// next to it.
//
// Nothing compared them. A Static cluster given a <Provider>ClusterConfiguration had that document
// written into the cluster as a d8-provider-cluster-configuration Secret, where the modules read
// it and configure a cloud provider for a cluster that has none; the other way round, a Cloud
// cluster given a StaticClusterConfiguration gets a d8-static-cluster-configuration Secret that
// contradicts the infrastructure dhctl is about to create. Both are silent, and both surface
// later as a cluster behaving as though it were something else.
func validateClusterTypeAgainstDocuments(m *MetaConfig) error {
	if len(m.ClusterConfig) == 0 {
		return nil
	}

	switch m.ClusterType {
	case CloudClusterType:
		if len(m.StaticClusterConfig) > 0 {
			return fmt.Errorf(
				"ClusterConfiguration.clusterType is %q and the configuration also carries a "+
					"StaticClusterConfiguration. A cloud cluster takes its node network from the provider, "+
					"and the two documents would describe it differently. Remove the StaticClusterConfiguration, "+
					"or set clusterType to Static",
				CloudClusterType)
		}

	case StaticClusterType:
		if len(m.ProviderClusterConfig) > 0 {
			return fmt.Errorf(
				"ClusterConfiguration.clusterType is %q and the configuration also carries a "+
					"<Provider>ClusterConfiguration. dhctl would write that document into the cluster, where the "+
					"modules would configure a cloud provider for a cluster that has none. Remove the "+
					"<Provider>ClusterConfiguration, or set clusterType to Cloud",
				StaticClusterType)
		}
	}

	return nil
}

// warnAboutMasterReplicaParity says what an even number of masters costs.
//
// etcd keeps quorum with a majority, so two masters tolerate no failures at all — the same as one,
// and with twice the machines to lose. Four tolerate one, the same as three. It is a warning
// rather than an error: an even count is a legitimate transient state while a cluster is being
// grown, and dhctl is not the place to refuse it.
func warnAboutMasterReplicaParity(ctx context.Context, m *MetaConfig) {
	replicas := m.MasterNodeGroupSpec.Replicas
	if replicas <= 1 || replicas%2 == 1 {
		return
	}

	dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
		"masterNodeGroup.replicas is %d. etcd keeps quorum with a majority, so %d masters tolerate the loss of "+
			"%d — the same as %d would, with one more machine to lose. Use an odd number",
		replicas, replicas, replicas/2-1, replicas-1))
}
