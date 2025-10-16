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

package app

import "gopkg.in/alecthomas/kingpin.v2"

var (
	PreflightSkipAll                          = false
	PreflightSkipSSHForward                   = false
	PreflightSkipAvailabilityPorts            = false
	PreflightSkipResolvingLocalhost           = false
	PreflightSkipDeckhouseEditionCheck        = false
	PreflightSkipRegistryThroughProxy         = false
	PreflightSkipPublicDomainTemplateCheck    = false
	PreflightSkipSSHCredentialsCheck          = false
	PreflightSkipRegistryCredentials          = false
	PreflightSkipContainerdExistCheck         = false
	PreflightSkipPythonChecks                 = false
	PreflightSkipSudoIsAllowedForUserCheck    = false
	PreflightSkipSystemRequirementsCheck      = false
	PreflightSkipOneSSHHost                   = false
	PreflightSkipCloudAPIAccessibility        = false
	PreflightSkipTimeDrift                    = false
	PreflightSkipCIDRIntersection             = false
	PreflightSkipDeckhouseUserCheck           = false
	PreflightSkipYandexWithNatInstanceCheck   = false
	PreflightSkipStaticInstancesIPDuplication = false
	PreflightSkipKernelEROFSCVE               = false
)

const (
	SSHForwardArgName                = "preflight-skip-ssh-forward-check"
	PortsAvailabilityArgName         = "preflight-skip-availability-ports-check"
	ResolvingLocalhostArgName        = "preflight-skip-resolving-localhost-check"
	DeckhouseEditionCheckArgName     = "preflight-skip-deckhouse-edition-check"
	RegistryThroughProxyCheckArgName = "preflight-skip-registry-through-proxy"
	PublicDomainTemplateCheckArgName = "preflight-skip-public-domain-template-check"
	SSHCredentialsCheckArgName       = "preflight-skip-ssh-credentials-check"
	RegistryCredentialsCheckArgName  = "preflight-skip-registry-credential"
	ContainerdExistCheckArgName      = "preflight-skip-containerd-exist"
	PythonChecksArgName              = "preflight-skip-python-checks"
	SudoAllowedCheckArgName          = "preflight-skip-sudo-allowed"
	SystemRequirementsArgName        = "preflight-skip-system-requirements-check"
	CloudAPIAccessibilityArgName     = "preflight-cloud-api-accesibility-check"
	TimeDriftArgName                 = "preflight-time-drift-check"
	OneSSHHostCheckArgName           = "preflight-skip-one-ssh-host"
	CIDRIntersection                 = "preflight-skip-cidr-intersection"
	DeckhouseUserCheckName           = "preflight-skip-deckhouse-user-check"
	YandexWithNatInstance            = "preflight-skip-yandex-with-nat-instance-check"
	StaticInstancesIPDuplication     = "preflight-skip-staticinstances-ip-duplication"
	KernelEROFSCVECheckArgName       = "preflight-skip-kernel-erofs-cve"
)

var PreflightSkipOptionsMap = map[string]*bool{
	SSHForwardArgName:                &PreflightSkipSSHForward,
	PortsAvailabilityArgName:         &PreflightSkipAvailabilityPorts,
	ResolvingLocalhostArgName:        &PreflightSkipResolvingLocalhost,
	DeckhouseEditionCheckArgName:     &PreflightSkipDeckhouseEditionCheck,
	RegistryThroughProxyCheckArgName: &PreflightSkipRegistryThroughProxy,
	PublicDomainTemplateCheckArgName: &PreflightSkipPublicDomainTemplateCheck,
	SSHCredentialsCheckArgName:       &PreflightSkipSSHCredentialsCheck,
	RegistryCredentialsCheckArgName:  &PreflightSkipRegistryCredentials,
	CloudAPIAccessibilityArgName:     &PreflightSkipCloudAPIAccessibility,
	ContainerdExistCheckArgName:      &PreflightSkipContainerdExistCheck,
	PythonChecksArgName:              &PreflightSkipPythonChecks,
	SudoAllowedCheckArgName:          &PreflightSkipSudoIsAllowedForUserCheck,
	SystemRequirementsArgName:        &PreflightSkipSystemRequirementsCheck,
	OneSSHHostCheckArgName:           &PreflightSkipOneSSHHost,
	CIDRIntersection:                 &PreflightSkipCIDRIntersection,
	StaticInstancesIPDuplication:     &PreflightSkipStaticInstancesIPDuplication,
	DeckhouseUserCheckName:           &PreflightSkipDeckhouseUserCheck,
	TimeDriftArgName:                 &PreflightSkipTimeDrift,
	YandexWithNatInstance:            &PreflightSkipYandexWithNatInstanceCheck,
	KernelEROFSCVECheckArgName:       &PreflightSkipKernelEROFSCVE,
}

func ApplyPreflightSkips(skips []string) {
	for _, skip := range skips {
		if arg, hasKey := PreflightSkipOptionsMap[skip]; hasKey {
			*arg = true
		}
	}
}

func DefinePreflight(cmd *kingpin.CmdClause) {
	cmd.Flag("preflight-skip-all-checks", "Skip all preflight checks").
		Envar(configEnvName("PREFLIGHT_SKIP_ALL_CHECKS")).
		BoolVar(&PreflightSkipAll)
	cmd.Flag(SSHForwardArgName, "Skip SSH forward preflight check").
		Envar(configEnvName("PREFLIGHT_SKIP_SSH_FORWARD_CHECK")).
		BoolVar(PreflightSkipOptionsMap[SSHForwardArgName])
	cmd.Flag(PortsAvailabilityArgName, "Skip availability ports preflight check").
		Envar(configEnvName("PREFLIGHT_SKIP_AVAILABILITY_PORTS_CHECK")).
		BoolVar(PreflightSkipOptionsMap[PortsAvailabilityArgName])
	cmd.Flag(ResolvingLocalhostArgName, "Skip resolving the localhost domain").
		Envar(configEnvName("PREFLIGHT_SKIP_RESOLVING_LOCALHOST_CHECK")).
		BoolVar(PreflightSkipOptionsMap[ResolvingLocalhostArgName])
	cmd.Flag(DeckhouseEditionCheckArgName, "Skip verifying deckhouse edition").
		Envar(configEnvName("PREFLIGHT_SKIP_INCOMPATIBLE_EDITION_CHECK")).
		BoolVar(PreflightSkipOptionsMap[DeckhouseEditionCheckArgName])
	cmd.Flag(RegistryThroughProxyCheckArgName, "Skip verifying deckhouse version").
		Envar(configEnvName("PREFLIGHT_SKIP_REGISTRY_THROUGH_PROXY")).
		BoolVar(PreflightSkipOptionsMap[RegistryThroughProxyCheckArgName])
	cmd.Flag(PublicDomainTemplateCheckArgName, "Skip verifying PublicDomainTemplate check").
		Envar(configEnvName("PREFLIGHT_SKIP_PUBLIC_DOMAIN_TEMPLATE")).
		BoolVar(PreflightSkipOptionsMap[PublicDomainTemplateCheckArgName])
	cmd.Flag(SSHCredentialsCheckArgName, "Skip verifying PublicDomainTemplate check").
		Envar(configEnvName("PREFLIGHT_SKIP_SSH_CREDENTIAL_CHECK")).
		BoolVar(PreflightSkipOptionsMap[SSHCredentialsCheckArgName])
	cmd.Flag(RegistryCredentialsCheckArgName, "Skip verifying registry credentials").
		Envar(configEnvName("PREFLIGHT_SKIP_REGISTRY_CREDENTIALS")).
		BoolVar(PreflightSkipOptionsMap[RegistryCredentialsCheckArgName])
	cmd.Flag(CloudAPIAccessibilityArgName, "Skip verifying Cloud API").
		Envar(configEnvName("PREFLIGHT_SKIP_CLOUD_API_CHECK")).
		BoolVar(PreflightSkipOptionsMap[CloudAPIAccessibilityArgName])
	cmd.Flag(TimeDriftArgName, "Skip verifying time drift").
		Envar(configEnvName("PREFLIGHT_SKIP_TIME_DRIFT")).
		BoolVar(PreflightSkipOptionsMap[TimeDriftArgName])
	cmd.Flag(ContainerdExistCheckArgName, "Skip verifying contanerd exist").
		Envar(configEnvName("PREFLIGHT_SKIP_CONTAINERD_EXIST")).
		BoolVar(PreflightSkipOptionsMap[ContainerdExistCheckArgName])
	cmd.Flag(PythonChecksArgName, "Skip verifying python installation").
		Envar(configEnvName("PREFLIGHT_SKIP_PYTHON_CHECKS")).
		BoolVar(PreflightSkipOptionsMap[PythonChecksArgName])
	cmd.Flag(SudoAllowedCheckArgName, "Skip verifying sudo is allowed for user").
		Envar(configEnvName("PREFLIGHT_SKIP_SUDO_ALLOWED_CHECK")).
		BoolVar(PreflightSkipOptionsMap[SudoAllowedCheckArgName])
	cmd.Flag(SystemRequirementsArgName, "Skip verifying system requirements").
		Envar(configEnvName("PREFLIGHT_SKIP_SYSTEM_REQUIREMENTS_CHECK")).
		BoolVar(PreflightSkipOptionsMap[SystemRequirementsArgName])
	cmd.Flag(OneSSHHostCheckArgName, "Skip verifying one ssh-host parametr").
		Envar(configEnvName("PREFLIGHT_SKIP_ONE_SSH_HOST")).
		BoolVar(PreflightSkipOptionsMap[OneSSHHostCheckArgName])
	cmd.Flag(CIDRIntersection, "Skip verifying CIDRs intersection").
		Envar(configEnvName("PREFLIGHT_SKIP_CIDR_INTERSECTION")).
		BoolVar(PreflightSkipOptionsMap[CIDRIntersection])
	cmd.Flag(DeckhouseUserCheckName, "Skip verifying deckhouse user existence").
		Envar(configEnvName("PREFLIGHT_SKIP_DECKHOUSE_USER")).
		BoolVar(PreflightSkipOptionsMap[DeckhouseUserCheckName])
	cmd.Flag(YandexWithNatInstance, "Skip verifying Yandex Cloud WithNatInstance configuration").
		Envar(configEnvName("PREFLIGHT_SKIP_YANDEX_WITH_NAT_INSTANCE_CHECK")).
		BoolVar(PreflightSkipOptionsMap[YandexWithNatInstance])
	cmd.Flag(StaticInstancesIPDuplication, "Skip verifying StaticInstances IP intersection").
		Envar(configEnvName("PREFLIGHT_SKIP_SI_IP_DUPLICATION")).
		BoolVar(PreflightSkipOptionsMap[StaticInstancesIPDuplication])
	cmd.Flag(KernelEROFSCVECheckArgName, "Skip verifying kernel CVE-2025-37999 (EROFS) vulnerability").
		Envar(configEnvName("PREFLIGHT_SKIP_KERNEL_EROFS_CVE")).
		BoolVar(PreflightSkipOptionsMap[KernelEROFSCVECheckArgName])
}
