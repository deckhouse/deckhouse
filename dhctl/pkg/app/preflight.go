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
	PreflightSkipAll                = false
	PreflightSkipSSHForword         = false
	PreflightSkipAvailabilityPorts  = false
	PreflightSkipResolvingLocalhost = false
)

func DefinePreflight(cmd *kingpin.CmdClause) {
	cmd.Flag("preflight-skip-all-checks", "Skip all preflight checks").
		Envar(configEnvName("PREFLIGHT_SKIP_ALL_CHECKS")).
		BoolVar(&PreflightSkipAll)
	cmd.Flag("preflight-skip-ssh-forward-check", "Skip SSH forward preflight check").
		Envar(configEnvName("PREFLIGHT_SKIP_SSH_FORWARD_CHECK")).
		BoolVar(&PreflightSkipSSHForword)
	cmd.Flag("preflight-skip-availability-ports-check", "Skip availability ports preflight check").
		Envar(configEnvName("PREFLIGHT_SKIP_AVAILABILITY_PORTS_CHECK")).
		BoolVar(&PreflightSkipAvailabilityPorts)
	cmd.Flag("preflight-skip-resolving-localhost-check", "Skip resolving the localhost domain").
		Envar(configEnvName("PREFLIGHT_SKIP_RESOLVING_LOCALHOST_CHECK")).
		BoolVar(&PreflightSkipResolvingLocalhost)
}
