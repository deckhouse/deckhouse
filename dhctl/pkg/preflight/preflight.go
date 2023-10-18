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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type Checker struct {
	sshClient               *ssh.Client
	metaConfig              *config.MetaConfig
	installConfig           *deckhouse.Config
	imageDescriptorProvider imageDescriptorProvider
	buildDigestProvider     buildDigestProvider
}

type preflightCheckFunc func() error

func NewChecker(sshClient *ssh.Client, config *deckhouse.Config, metaConfig *config.MetaConfig) Checker {
	return Checker{
		sshClient:               sshClient,
		metaConfig:              metaConfig,
		installConfig:           config,
		imageDescriptorProvider: remoteDescriptorProvider{},
		buildDigestProvider:     &dhctlBuildDigestProvider{DigestFilePath: app.DeckhouseImageDigestFile},
	}
}

func (pc *Checker) Static() error {
	return pc.do("Preflight checks for static-cluster", []preflightCheckFunc{
		pc.CheckSSHTunel,
		pc.CheckRegistryAccessThroughProxy,
		pc.CheckAvailabilityPorts,
		pc.CheckLocalhostDomain,
	})
}

func (pc *Checker) Cloud() error {
	return nil
}

func (pc *Checker) Global() error {
	return pc.do("Global preflight checks", []preflightCheckFunc{
		pc.CheckDhctlVersionObsolescence,
	})
}

func (pc *Checker) do(title string, checks []preflightCheckFunc) error {
	return log.Process("common", title, func() error {
		if app.PreflightSkipAll {
			log.InfoLn("Preflight checks were skipped")
			return nil
		}

		for _, checkFunc := range checks {
			if err := checkFunc(); err != nil {
				return fmt.Errorf(`Installation aborted:
%w
Please fix this problem or skip if you're sure (please see help for find necessary flag)`, err)
			}
		}

		return nil
	})
}
