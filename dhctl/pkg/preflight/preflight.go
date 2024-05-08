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
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Checker struct {
	sshClient               *ssh.Client
	metaConfig              *config.MetaConfig
	installConfig           *config.DeckhouseInstaller
	imageDescriptorProvider imageDescriptorProvider
	buildDigestProvider     buildDigestProvider
}

type checkStep struct {
	successMessage string
	skipFlag       string
	fun            func() error
}

func NewChecker(sshClient *ssh.Client, config *config.DeckhouseInstaller, metaConfig *config.MetaConfig) Checker {
	return Checker{
		sshClient:               sshClient,
		metaConfig:              metaConfig,
		installConfig:           config,
		imageDescriptorProvider: remoteDescriptorProvider{},
		buildDigestProvider:     &dhctlBuildDigestProvider{DigestFilePath: app.DeckhouseImageDigestFile},
	}
}

func (pc *Checker) Static() error {
	return pc.do("Preflight checks for static-cluster", []checkStep{
		{
			fun:            pc.CheckSSHTunnel,
			successMessage: "ssh tunnel will up",
			skipFlag:       app.SSHForwardArgName,
		},
		{
			fun:            pc.CheckRegistryAccessThroughProxy,
			successMessage: "registry access through proxy",
			skipFlag:       app.RegistryThroughProxyCheckArgName,
		},
		{
			fun:            pc.CheckAvailabilityPorts,
			successMessage: "required ports availability",
			skipFlag:       app.PortsAvailabilityArgName,
		},
		{
			fun:            pc.CheckLocalhostDomain,
			successMessage: "resolve the localhost domain",
			skipFlag:       app.ResolvingLocalhostArgName,
		},
	})
}

func (pc *Checker) Cloud() error {
	return nil
}

func (pc *Checker) Global() error {
	return pc.do("Global preflight checks", []checkStep{
		{
			fun:            pc.CheckPublicDomainTemplate,
			successMessage: "PublicDomainTemplate is correctly",
			skipFlag:       app.PublicDomainTemplateCheckArgName,
		},
	})
}

func (pc *Checker) do(title string, checks []checkStep) error {
	return log.Process("common", title, func() error {
		if app.PreflightSkipAll {
			log.WarnLn("Preflight checks were skipped")
			return nil
		}

		for _, check := range checks {
			loop := retry.NewLoop(fmt.Sprintf("Checking %s", check.successMessage), 1, 10*time.Second)
			if err := loop.Run(check.fun); err != nil {
				return fmt.Errorf("Installation aborted: %w\n"+
					`Please fix this problem or skip it if you're sure with %s flag`, err, check.skipFlag)
			}
		}

		return nil
	})
}
