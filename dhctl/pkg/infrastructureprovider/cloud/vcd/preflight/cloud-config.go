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

package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type ConfigDeps struct {
	MetaConfig      *config.MetaConfig
	ValidatePrefix  bool
	CheckServerPath bool
}

type configCheck struct {
	deps ConfigDeps
}

type providerConfig struct {
	Server   string `json:"server"`
	Insecure bool   `json:"insecure,omitempty"`
}

const ConfigCheckName preflightnew.CheckName = "vcd-cloud-config"

func (configCheck) Description() string {
	return "validate vcd provider configuration"
}

func (configCheck) Phase() preflightnew.Phase {
	return preflightnew.PhaseProviderConfigCheck
}

func (configCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c configCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.deps.MetaConfig == nil {
		return fmt.Errorf("meta config is nil")
	}

	if c.deps.ValidatePrefix {
		if err := validation.DefaultPrefixValidator(c.deps.MetaConfig.ClusterPrefix); err != nil {
			return fmt.Errorf("%v for provider %s", err, c.deps.MetaConfig.ProviderName)
		}
	}

	if !c.deps.CheckServerPath {
		return nil
	}

	var providerConfiguration providerConfig
	if err := json.Unmarshal(c.deps.MetaConfig.ProviderClusterConfig["provider"], &providerConfiguration); err != nil {
		return fmt.Errorf("unable to unmarshal vcd provider configuration: %v", err)
	}

	server := strings.TrimSpace(providerConfiguration.Server)
	if server == "" {
		return nil
	}

	if strings.HasSuffix(server, "/") {
		return fmt.Errorf("provider.server must not end with a slash '/'")
	}

	return nil
}

func ConfigCheck(deps ConfigDeps) preflightnew.Check {
	check := configCheck{deps: deps}
	return preflightnew.Check{
		Name:        ConfigCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
