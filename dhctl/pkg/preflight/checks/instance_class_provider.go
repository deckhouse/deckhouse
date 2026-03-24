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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type InstanceClassProviderCheck struct {
	MetaConfig *config.MetaConfig
}

const InstanceClassProviderCheckName preflight.CheckName = "instance-class-provider"

func (InstanceClassProviderCheck) Description() string {
	return "instance classes match current cloud provider"
}

func (InstanceClassProviderCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (InstanceClassProviderCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c InstanceClassProviderCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}

	validator := infrastructureprovider.NewInstanceClassValidator(c.MetaConfig)
	return validator.ValidateProviderInstanceClasses()
}

func InstanceClassProvider(metaConfig *config.MetaConfig) preflight.Check {
	check := InstanceClassProviderCheck{MetaConfig: metaConfig}
	return preflight.Check{
		Name:        InstanceClassProviderCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
