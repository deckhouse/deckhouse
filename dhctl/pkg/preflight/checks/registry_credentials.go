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
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type RegistryCredentialsCheck struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller

	descriptor imageDescriptorProvider
}

const RegistryCredentialsCheckName preflight.CheckName = "registry-credentials"

func (RegistryCredentialsCheck) Description() string {
	return "deckhouse image is available in registry"
}

func (RegistryCredentialsCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (RegistryCredentialsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

func (c RegistryCredentialsCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil || c.InstallConfig == nil {
		return fmt.Errorf("metaConfig and installConfig are required")
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	_, err := deckhouseImageConfig(
		ctx,
		c.MetaConfig,
		c.InstallConfig,
		c.descriptor,
	)
	if err != nil {
		return fmt.Errorf("cannot resolve deckhouse image config: %w", err)
	}

	return nil
}

func RegistryCredentials(meta *config.MetaConfig, cfg *config.DeckhouseInstaller) preflight.Check {
	check := RegistryCredentialsCheck{
		MetaConfig:    meta,
		InstallConfig: cfg,
	}

	return preflight.Check{
		Name:        RegistryCredentialsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
