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

	log.DebugLn("Checking resolving the localhost domain")

	file, err := template.RenderAndSavePreflightCheckLocalhostScript()
	if err != nil {
		return err
	}

	scriptCmd := pc.sshClient.UploadScript(file)
	out, err := scriptCmd.Execute()
	if err != nil {
		log.ErrorLn(strings.Trim(string(out), "\n"))
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("check_localhost.sh: %w, %s", err, string(ee.Stderr))
		}
		return fmt.Errorf("check_localhost.sh: %w", err)
	}

	log.DebugLn(string(out))
	return nil
}
