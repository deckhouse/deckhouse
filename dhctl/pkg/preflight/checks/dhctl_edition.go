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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type DhctlEditionCheck struct {
	MetaConfig *config.MetaConfig
	Installer  *config.DeckhouseInstaller
	BuildInfo  options.BuildInfo

	descriptor imageDescriptorProvider
}

const DhctlEditionCheckName preflight.CheckName = "dhctl-edition"

func (DhctlEditionCheck) Description() string {
	return "dhctl edition matches deckhouse image"
}

func (DhctlEditionCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (DhctlEditionCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.RetryPolicy{Attempts: 1}
}

func (c DhctlEditionCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil || c.Installer == nil {
		return fmt.Errorf("metaConfig and installConfig are required")
	}

	imageConfig, err := deckhouseImageConfig(
		ctx,
		c.MetaConfig,
		c.Installer,
		c.descriptor,
	)
	if err != nil {
		return fmt.Errorf("cannot fetch deckhouse image config: %w", err)
	}

	labels := imageConfig.Config.Labels
	if labels == nil || labels["io.deckhouse.edition"] != c.BuildInfo.AppEdition {
		return fmt.Errorf(
			"your edition installer image does not match: dhctl edition %s, image edition %s",
			c.BuildInfo.AppEdition,
			labels["io.deckhouse.edition"],
		)
	}

	return nil
}

func DhctlEdition(
	meta *config.MetaConfig,
	cfg *config.DeckhouseInstaller,
	buildInfo options.BuildInfo,
) preflight.Check {
	check := DhctlEditionCheck{
		MetaConfig: meta,
		Installer:  cfg,
		BuildInfo:  buildInfo,
	}

	preflightCheck := preflight.Check{
		Name:        DhctlEditionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}

	if buildInfo.AppVersion == "local" || buildInfo.AppEdition == "local" {
		preflightCheck.Disable()
	}

	return preflightCheck
}
