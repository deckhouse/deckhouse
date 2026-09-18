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

package checks

// Reading the cluster CIDRs out of the configuration.
//
// The checks that used to live here — cidr-intersection and static-cidr-intersection — are part
// of loading the configuration now (pkg/config/cluster_network_validation.go): they read nothing
// but the documents, so they belong where a bad document is refused rather than where a network
// is probed. What is left is used by the checks that compare those CIDRs against something only
// a node or a cloud can tell us: the networks of a real node, and the network a cloud will put
// the nodes on.

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// getCIDRs reads the resolved pair, wherever the operator declared it.
//
// Through MetaConfig.Network(), not out of ClusterConfig: #22688 moved these parameters to
// ModuleConfig control-plane-manager and left the ClusterConfiguration fields as a deprecated
// fallback, so reading the document directly returns "" for every cluster that has migrated —
// and an empty string compares against a node's networks without overlapping anything, which is
// a check that passes by finding nothing.
func getCIDRs(meta *config.MetaConfig) (string, string, error) {
	network := meta.Network()

	if network.PodSubnetCIDR == "" || network.ServiceSubnetCIDR == "" {
		return "", "", fmt.Errorf(
			"podSubnetCIDR and serviceSubnetCIDR must be set either in ModuleConfig " +
				"control-plane-manager (spec.settings.network) or in ClusterConfiguration (deprecated)")
	}

	return network.PodSubnetCIDR, network.ServiceSubnetCIDR, nil
}

// The parameter is named bare rather than qualified with a document: since #22688 it may be
// declared in either, and the fix names both so the reader edits the one they actually have.
func invalidCIDRFailure(name, cidr string) error {
	return preflight.Permanent(&preflight.Failure{
		Checked:  name,
		Observed: fmt.Sprintf("%q is not a CIDR", cidr),
		Expected: "an address and a prefix length, for example 10.111.0.0/16",
		Fix: fmt.Sprintf("correct %s in ModuleConfig control-plane-manager (spec.settings.network), "+
			"or in ClusterConfiguration if it is still declared there", name),
	})
}
