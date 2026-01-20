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
package checks

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type CidrIntersectionCheck struct {
	MetaConfig *config.MetaConfig
}

const CidrIntersectionCheckName preflightnew.CheckName = "cidr-intersection"

func (CidrIntersectionCheck) Description() string {
	return "cluster CIDRs do not intersect"
}

func (CidrIntersectionCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (CidrIntersectionCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (CidrIntersectionCheck) Enabled() bool {
	return true
}

func (c CidrIntersectionCheck) Run(ctx context.Context) error {
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

	return cidrIntersects(podCIDR, serviceCIDR)
}

func CidrIntersection(meta *config.MetaConfig) preflightnew.Check {
	check := CidrIntersectionCheck{MetaConfig: meta}
	return preflightnew.Check{
		Name:        CidrIntersectionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
