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
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Checker struct {
	nodeInterface           node.Interface
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

func NewChecker(
	nodeInterface node.Interface,
	config *config.DeckhouseInstaller,
	metaConfig *config.MetaConfig,
) Checker {
	return Checker{
		nodeInterface:           nodeInterface,
		metaConfig:              metaConfig,
		installConfig:           config,
		imageDescriptorProvider: remoteDescriptorProvider{},
		buildDigestProvider: &dhctlBuildDigestProvider{
			DigestFilePath: app.DeckhouseImageDigestFile,
		},
	}
}

func (pc *Checker) Static() error {
	return pc.do("Preflight checks for static-cluster", []checkStep{
		{
			fun:            pc.CheckSingleSSHHostForStatic,
			successMessage: "only one --ssh-host parameter used",
			skipFlag:       app.OneSSHHostCheckArgName,
		},
		{
			fun:            pc.CheckSSHCredential,
			successMessage: "ssh credential is correctly",
			skipFlag:       app.SSHCredentialsCheckArgName,
		},
		{
			fun:            pc.CheckSSHTunnel,
			successMessage: "ssh tunnel between installer and node is possible",
			skipFlag:       app.SSHForwardArgName,
		},
		{
			fun:            pc.CheckStaticNodeSystemRequirements,
			successMessage: "that node meets system requirements",
			skipFlag:       app.SystemRequirementsArgName,
		},
		{
			fun:            pc.CheckPythonAndItsModules,
			successMessage: "python and required modules are installed",
			skipFlag:       app.PythonChecksArgName,
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
			skipFlag:       app.RegistryCredentialsCheckArgName,
		},
		{
			fun:            pc.CheckSudoIsAllowedForUser,
			successMessage: "sudo is allowed for user",
			skipFlag:       app.SudoAllowedCheckArgName,
		},
	})
}

func (pc *Checker) Cloud() error {
	return pc.do("Cloud deployment preflight checks", []checkStep{
		{
			fun:            pc.CheckCloudMasterNodeSystemRequirements,
			successMessage: "cloud master node system requirements are met",
			skipFlag:       app.SystemRequirementsArgName,
		},
	})
}

func (pc *Checker) Global() error {
	return pc.do("Global preflight checks", []checkStep{
		{
			fun:            pc.CheckPublicDomainTemplate,
			successMessage: "PublicDomainTemplate is correctly",
			skipFlag:       app.PublicDomainTemplateCheckArgName,
		},
		{
			fun:            pc.CheckRegistryCredentials,
			successMessage: "registry credentials are correct",
			skipFlag:       app.RegistryCredentialsCheckArgName,
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
			loop := retry.NewLoop(
				fmt.Sprintf("Checking %s", check.successMessage),
				1,
				10*time.Second,
			)
			if err := loop.Run(check.fun); err != nil {
				return fmt.Errorf("Installation aborted: %w\n"+
					`Please fix this problem or skip it if you're sure with %s flag`, err, check.skipFlag)
			}
		}

		return nil
	})
}
