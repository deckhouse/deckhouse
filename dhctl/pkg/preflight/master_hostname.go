// Copyright 2024 Flant JSC
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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func (pc *Checker) CheckMasterHostname() error {
	if app.PreflightSkipMasterHostname {
		log.InfoLn("Master hostname preflight check was skipped")
		return nil
	}

	if pc.sshClient.Settings.CountHosts() < 2 {
		log.DebugLn("Master hostname preflight check was skipped")
		return nil
	}
	log.DebugLn("Checking if localhost domain resolves correctly")

	file, err := template.RenderAndSavePreflightCheckScript("get_hostname.sh", nil)
	if err != nil {
		return err
	}

	masterHostnames := make(map[string]struct{})
	masterWithError := make([]string, 0)

	err = pc.sshClient.Loop(func(sshClient *ssh.Client) error {
		log.DebugF("Get hostname from master %s\n", sshClient.Settings.Host())
		scriptCmd := sshClient.UploadScript(file)
		out, err := scriptCmd.Execute()
		if err != nil {
			log.ErrorLn(strings.Trim(string(out), "\n"))
			return fmt.Errorf(
				"could not execute a script to get master hostname: %w",
				err,
			)
		}
		hostname := strings.Trim(string(out), "\n")
		log.DebugF("Master: %s hostname: %s\n", sshClient.Settings.Host(), hostname)
		if _, ok := masterHostnames[hostname]; ok {
			log.ErrorF("Master with hostname %s already exist!\n", hostname)
			masterWithError = append(masterWithError, sshClient.Settings.Host())
			return nil
		}

		masterHostnames[hostname] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}

	if len(masterWithError) > 0 {
		return fmt.Errorf(
			"please set unique hostname on the servers %s and re-install the installation again",
			strings.Join(masterWithError, ","),
		)
	}

	return nil
}
