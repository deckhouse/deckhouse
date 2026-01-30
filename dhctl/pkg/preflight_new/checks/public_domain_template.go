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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type PublicDomainTemplateCheck struct {
	MetaConfig *config.MetaConfig
}

const PublicDomainTemplateCheckName preflightnew.CheckName = "public-domain-template"

func (PublicDomainTemplateCheck) Description() string {
	return "publicDomainTemplate does not match clusterDomain"
}

func (PublicDomainTemplateCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (PublicDomainTemplateCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (PublicDomainTemplateCheck) Enabled() bool {
	return true
}

func (c PublicDomainTemplateCheck) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, preflightnew.DefaultPreflightCheckTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if c.MetaConfig == nil {
		return fmt.Errorf("metaConfig is required")
	}

	for _, mc := range c.MetaConfig.ModuleConfigs {
		if mc.GetName() != "global" {
			continue
		}

		clusterDomain, err := clusterDomain(c.MetaConfig)
		if err != nil {
			return err
		}

		templateValue, err := publicDomainTemplate(mc.Spec.Settings["modules"])
		if err != nil {
			return err
		}

		if strings.Contains(templateValue, clusterDomain) {
			return fmt.Errorf("the publicDomainTemplate %q must not match clusterDomain %q", templateValue, clusterDomain)
		}
	}

	return nil
}

func publicDomainTemplate(settings any) (string, error) {
	var payload struct {
		PublicDomainTemplate string `json:"publicDomainTemplate,omitempty"`
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(settingsJSON, &payload); err != nil {
		return "", err
	}

	return payload.PublicDomainTemplate, nil
}

func clusterDomain(meta *config.MetaConfig) (string, error) {
	var domain string

	if err := json.Unmarshal(meta.ClusterConfig["clusterDomain"], &domain); err != nil {
		return "", err
	}

	return domain, nil
}

func PublicDomainTemplate(meta *config.MetaConfig) preflightnew.Check {
	check := PublicDomainTemplateCheck{MetaConfig: meta}
	return preflightnew.Check{
		Name:        PublicDomainTemplateCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
