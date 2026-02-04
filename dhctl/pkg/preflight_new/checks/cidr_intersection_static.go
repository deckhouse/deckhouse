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

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type CidrIntersectionStaticCheck struct {
	MetaConfig *config.MetaConfig
}

const CidrIntersectionStaticCheckName preflightnew.CheckName = "static-cidr-intersection"

func (CidrIntersectionStaticCheck) Description() string {
	return "cluster CIDRs do not intersect with static internal networks"
}

func (CidrIntersectionStaticCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (CidrIntersectionStaticCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c CidrIntersectionStaticCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, preflightnew.DefaultPreflightCheckTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.MetaConfig == nil {
		return fmt.Errorf("metaConfig is required")
	}

	podCIDR, serviceCIDR, err := getCIDRs(c.MetaConfig)
	if err != nil {
		return err
	}

	internalCIDRs, err := internalNetworkCIDRs(c.MetaConfig)
	if err != nil {
		return err
	}

	for _, internal := range internalCIDRs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cidrIntersects(podCIDR, internal); err != nil {
			return err
		}
		if err := cidrIntersects(serviceCIDR, internal); err != nil {
			return err
		}
	}

	return nil
}

func getCIDRs(meta *config.MetaConfig) (string, string, error) {
	var podCIDR string
	var serviceCIDR string

	if err := json.Unmarshal(meta.ClusterConfig["podSubnetCIDR"], &podCIDR); err != nil {
		return "", "", fmt.Errorf("missing podSubnetCIDR field in ClusterConfiguration")
	}

	if err := json.Unmarshal(meta.ClusterConfig["serviceSubnetCIDR"], &serviceCIDR); err != nil {
		return "", "", fmt.Errorf("missing serviceSubnetCIDR field in ClusterConfiguration")
	}

	return podCIDR, serviceCIDR, nil
}

func internalNetworkCIDRs(meta *config.MetaConfig) ([]string, error) {
	cidrsData, ok := meta.StaticClusterConfig["internalNetworkCIDRs"]
	if !ok || cidrsData == nil {
		return nil, nil
	}

	var cidrs []string
	if err := json.Unmarshal(cidrsData, &cidrs); err != nil {
		return nil, fmt.Errorf("missing internalNetworkCIDRs field in ClusterConfiguration")
	}
	return cidrs, nil
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

func CidrIntersectionStatic(meta *config.MetaConfig) preflightnew.Check {
	check := CidrIntersectionStaticCheck{MetaConfig: meta}
	return preflightnew.Check{
		Name:        CidrIntersectionStaticCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
