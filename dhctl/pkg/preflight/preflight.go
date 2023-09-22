// Copyright 2023 Flant JSC
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

package preflight

import (
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type PreflightCheck struct {
	sshClient               *ssh.Client
	metaConfig              *config.MetaConfig
	installConfig           *deckhouse.Config
	imageDescriptorProvider imageDescriptorProvider
	buildDigestProvider     buildDigestProvider
}

func NewPreflightCheck(sshClient *ssh.Client, config *deckhouse.Config, metaConfig *config.MetaConfig) PreflightCheck {
	return PreflightCheck{
		sshClient:               sshClient,
		metaConfig:              metaConfig,
		installConfig:           config,
		imageDescriptorProvider: remoteDescriptorProvider{},
		buildDigestProvider:     &dhctlBuildDigestProvider{DigestFilePath: app.DeckhouseImageDigestFile},
	}
}

func (pc *PreflightCheck) StaticCheck() error {
	return log.Process("common", "Preflight Checks", func() error {
		if app.PreflightSkipAll {
			log.InfoLn("Preflight checks were skipped")
			return nil
		}

		type preflightCheckFunc func() error
		checks := []preflightCheckFunc{
			pc.CheckDhctlVersionObsolescence,
			pc.CheckRegistryAccessThroughProxy,
			pc.CheckSSHTunel,
			pc.CheckAvailabilityPorts,
			pc.CheckLocalhostDomain,
		}

		for _, checkFunc := range checks {
			if err := checkFunc(); err != nil {
				return err
			}
		}

		return nil
	})
}

func (pc *PreflightCheck) CloudCheck() error {
	return nil
}
