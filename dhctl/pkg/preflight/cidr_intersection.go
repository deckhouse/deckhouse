// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type internalNetworkCIDRs struct {
	internalNetworkCIDR []string
}

func (pc *Checker) CheckCidrIntersection() error {
	podSubnetCIDR, serviceSubnetCIDR, err := getCidrFromMetaConfig(pc.metaConfig)
	if err != nil {
		return err
	}

	err = cidrIntersects(podSubnetCIDR, serviceSubnetCIDR)
	if err != nil {
		return err
	}

	return nil
}

func (pc *Checker) CheckCidrIntersectionStatic() error {
	podSubnetCIDR, serviceSubnetCIDR, err := getCidrFromMetaConfig(pc.metaConfig)
	if err != nil {
		return err
	}

	var internalNetworkCIDRs internalNetworkCIDRs
	err = json.Unmarshal(pc.metaConfig.StaticClusterConfig["internalNetworkCIDRs"], &internalNetworkCIDRs)
	if err != nil {
		return err
	}

	err = cidrIntersects(podSubnetCIDR, serviceSubnetCIDR)
	if err != nil {
		return err
	}

	for _, ininternalNetworkCIDR := range internalNetworkCIDRs.internalNetworkCIDR {
		err := cidrIntersects(podSubnetCIDR, ininternalNetworkCIDR)
		if err != nil {
			return err
		}
		err = cidrIntersects(serviceSubnetCIDR, ininternalNetworkCIDR)
		if err != nil {
			return err
		}
	}

	return nil
}

func cidrIntersects(cidr1, cidr2 string) error {
	_, ipNet1, err := net.ParseCIDR(cidr1)
	if err != nil {
		return fmt.Errorf("invalid CIDR address: %s", cidr1)
	}

	_, ipNet2, err := net.ParseCIDR(cidr2)
	if err != nil {
		return fmt.Errorf("invalid CIDR address: %s", cidr2)
	}

	if ipNet1.Contains(ipNet2.IP) || ipNet2.Contains(ipNet1.IP) {
		return fmt.Errorf("CIDRs %s and %s are intersects", cidr1, cidr2)
	}
	return nil
}

func getCidrFromMetaConfig(metaConfig *config.MetaConfig) (string, string, error) {
	podSubnetCIDR := fmt.Sprint(metaConfig.ClusterConfig["podSubnetCIDR"])
	serviceSubnetCIDR := fmt.Sprint(metaConfig.ClusterConfig["serviceSubnetCIDR"])

	return podSubnetCIDR, serviceSubnetCIDR, nil
}
