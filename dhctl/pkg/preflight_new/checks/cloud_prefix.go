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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type CloudPrefixCheck struct {
	MetaConfig *config.MetaConfig
}

const CloudPrefixCheckName preflightnew.CheckName = "cloud-prefix"

func (CloudPrefixCheck) Description() string {
	return "validate cluster prefix for cloud provider"
}

func (CloudPrefixCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (CloudPrefixCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c CloudPrefixCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, preflightnew.DefaultPreflightCheckTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}
	if err := validation.DefaultPrefixValidator(c.MetaConfig.ClusterPrefix); err != nil {
		return fmt.Errorf("%v for provider %s", err, c.MetaConfig.ProviderName)
	}
	return nil
}

func CloudPrefix(metaConfig *config.MetaConfig) preflightnew.Check {
	check := CloudPrefixCheck{MetaConfig: metaConfig}
	return preflightnew.Check{
		Name:        CloudPrefixCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
