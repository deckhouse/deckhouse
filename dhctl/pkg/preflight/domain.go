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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func (pc *Checker) CheckLocalhostDomain() error {
	if app.PreflightSkipResolvingLocalhost {
		log.InfoLn("Resolving the localhost domain preflight check was skipped")
		return nil
	}

	log.DebugLn("Checking if localhost domain resolves correctly")

	file, err := template.RenderAndSavePreflightCheckLocalhostScript()
	if err != nil {
		return err
	}

	scriptCmd := pc.sshClient.UploadScript(file)
	out, err := scriptCmd.Execute()
	if err != nil {
		log.ErrorLn(strings.Trim(string(out), "\n"))
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("Localhost domain resolving check failed: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("Could not execute a script to check for localhost domain resolution: %w", err)
	}

	log.DebugLn(string(out))
	return nil
}

func (pc *Checker) CheckPublicDomainTemplate() error {
	if app.PreflightSkipPublicDomainTemplateCheck {
		log.InfoLn("PublicDomainTemplate preflight check was skipped")
		return nil
	}

	log.DebugLn("Checking if publicDomainTemplate was set correctly")

	for _, mc := range pc.metaConfig.ModuleConfigs {
		if mc.GetName() != "global" {
			continue
		}

		type SettingsModules struct {
			PublicDomainTemplate string `json:"publicDomainTemplate,omitempty"`
		}

		var (
			clusterDomain   string
			settingsModules SettingsModules
		)

		stringData, err := json.Marshal(mc.Spec.Settings["modules"])
		if err != nil {
			return err
		}
		err = json.Unmarshal(stringData, &settingsModules)
		if err != nil {
			return err
		}

		err = json.Unmarshal(pc.metaConfig.ClusterConfig["clusterDomain"], &clusterDomain)
		if err != nil {
			return err
		}

		if strings.Contains(settingsModules.PublicDomainTemplate, clusterDomain) {
			return fmt.Errorf("The publicDomainTemplate \"%s\" MUST NOT match the one specified in the clusterDomain parameter of the ClusterConfiguration resource: \"%s\".", settingsModules.PublicDomainTemplate, clusterDomain)
		}
	}
	return nil
}
