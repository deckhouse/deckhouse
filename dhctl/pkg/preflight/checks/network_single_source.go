// Copyright 2026 Flant JSC
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
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type NetworkSingleSourceCheck struct {
	MetaConfig *config.MetaConfig
}

const NetworkSingleSourceCheckName preflight.CheckName = "network-single-source"

func (NetworkSingleSourceCheck) Description() string {
	return "cluster network parameters are declared in only one of ClusterConfiguration or ModuleConfig control-plane-manager"
}

func (NetworkSingleSourceCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (NetworkSingleSourceCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c NetworkSingleSourceCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("metaConfig is required")
	}

	return c.MetaConfig.RequireNetworkSingleSource()
}

func NetworkSingleSource(meta *config.MetaConfig) preflight.Check {
	check := NetworkSingleSourceCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        NetworkSingleSourceCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
