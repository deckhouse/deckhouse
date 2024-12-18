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
	"errors"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type State interface {
	SetGlobalPreflightchecksWasRan() error
	GlobalPreflightchecksWasRan() (bool, error)
	SetCloudPreflightchecksWasRan() error
	SetPostCloudPreflightchecksWasRan() error
	CloudPreflightchecksWasRan() (bool, error)
	PostCloudPreflightchecksWasRan() (bool, error)
	SetStaticPreflightchecksWasRan() error
	StaticPreflightchecksWasRan() (bool, error)
}

type Checker struct {
	nodeInterface           node.Interface
	metaConfig              *config.MetaConfig
	installConfig           *config.DeckhouseInstaller
	bootstrapState          State
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
	bootstrapState State,
) Checker {
	return Checker{
		nodeInterface:           nodeInterface,
		metaConfig:              metaConfig,
		installConfig:           config,
		bootstrapState:          bootstrapState,
		imageDescriptorProvider: remoteDescriptorProvider{},
		buildDigestProvider: &dhctlBuildDigestProvider{
			DigestFilePath: app.DeckhouseImageDigestFile,
		},
	}
}

func (pc *Checker) Static() error {

	ready, err := pc.bootstrapState.StaticPreflightchecksWasRan()

	if err != nil {
		msg := fmt.Sprintf("Can not get state from cache: %v", err)
		return errors.New(msg)
	}

	if ready {
		return nil
	}

	err = pc.do("Preflight checks for static-cluster", []checkStep{
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
			skipFlag:       app.ResolvingLocalhostArgName,
		},
		{
			fun:            pc.CheckSudoIsAllowedForUser,
			successMessage: "sudo is allowed for user",
			skipFlag:       app.SudoAllowedCheckArgName,
		},
	})

	if err != nil {
		return err
	}

	return pc.bootstrapState.SetStaticPreflightchecksWasRan()
}

func (pc *Checker) Cloud() error {

	ready, err := pc.bootstrapState.CloudPreflightchecksWasRan()

	if err != nil {
		msg := fmt.Sprintf("Can not get state from cache: %v", err)
		return errors.New(msg)
	}

	if ready {
		return nil
	}

	err = pc.do("Cloud deployment preflight checks", []checkStep{
		{
			fun:            pc.CheckCloudMasterNodeSystemRequirements,
			successMessage: "cloud master node system requirements are met",
			skipFlag:       app.SystemRequirementsArgName,
		},
	})

	if err != nil {
		return err
	}

	return pc.bootstrapState.SetCloudPreflightchecksWasRan()

}

func (pc *Checker) PostCloud() error {

	ready, err := pc.bootstrapState.PostCloudPreflightchecksWasRan()

	if err != nil {
		msg := fmt.Sprintf("Can not get state from cache: %v", err)
		return errors.New(msg)
	}

	if ready {
		return nil
	}

	err = pc.do("Cloud deployment preflight checks", []checkStep{
		{
			fun:            pc.CheckCloudAPIAccessibility,
			successMessage: "access to cloud api from master host",
			skipFlag:       app.CloudAPIAccessibilityArgName,
		},
	})

	if err != nil {
		return err
	}

	return pc.bootstrapState.SetPostCloudPreflightchecksWasRan()

}

func (pc *Checker) Global() error {
	ready, err := pc.bootstrapState.GlobalPreflightchecksWasRan()

	if err != nil {
		msg := fmt.Sprintf("Can not get state from cache: %v", err)
		return errors.New(msg)
	}

	if ready {
		return nil
	}

	err = pc.do("Global preflight checks", []checkStep{
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

	if err != nil {
		return err
	}

	return pc.bootstrapState.SetGlobalPreflightchecksWasRan()

}

func (pc *Checker) do(title string, checks []checkStep) error {
	return log.Process("common", title, func() error {
		if app.PreflightSkipAll {
			log.WarnLn("Preflight checks were skipped")
			return nil
		}

		knownSkipFlags := make(map[string]struct{})
		for _, check := range checks {
			if _, skipFlagDuplicated := knownSkipFlags[check.skipFlag]; skipFlagDuplicated {
				panic("duplicated skip flag " + check.skipFlag)
			}
			knownSkipFlags[check.skipFlag] = struct{}{}

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
