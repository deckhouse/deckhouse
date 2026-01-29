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
	"errors"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type RegistryCredentialsCheck struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller
}

const RegistryCredentialsCheckName preflightnew.CheckName = "registry-credentials"

func (RegistryCredentialsCheck) Description() string {
	return "registry credentials are valid"
}

func (RegistryCredentialsCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (RegistryCredentialsCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (RegistryCredentialsCheck) Enabled() bool {
	return true
}

func (c RegistryCredentialsCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil || c.InstallConfig == nil {
		return fmt.Errorf("metaConfig and installConfig are required")
	}

	image := c.InstallConfig.GetRemoteImage(true)
	if image == "registry.deckhouse.ru/deckhouse/ce" {
		return nil
	}

	client, err := prepareAuthHTTPClient(c.MetaConfig)
	if err != nil {
		return err
	}

	authData := c.MetaConfig.Registry.Settings.RemoteData.AuthBase64()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := checkBasicRegistryAuth(ctx, c.MetaConfig, authData, client); err == nil {
		return nil
	} else if !errors.Is(err, ErrAuthRegistryFailed) {
		return err
	}

	return checkTokenRegistryAuth(ctx, c.MetaConfig, authData, client)
}

func RegistryCredentials(meta *config.MetaConfig, cfg *config.DeckhouseInstaller) preflightnew.Check {
	check := RegistryCredentialsCheck{
		MetaConfig:    meta,
		InstallConfig: cfg,
	}
	return preflightnew.Check{
		Name:        RegistryCredentialsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Enabled:     check.Enabled,
		Run:         check.Run,
	}
}
