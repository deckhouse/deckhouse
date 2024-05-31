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

	file, err := template.RenderAndSavePreflightCheckScript("get_hostname.sh", nil)
	if err != nil {
		return err
	}

	serverHostnames := make(map[string]struct{})
	serversWithError := make([]string, 0)

	err = pc.sshClient.Loop(func(sshClient *ssh.Client) error {
		var out []byte
		log.DebugF("Get hostname from server %s\n", sshClient.Settings.Host())
		scriptCmd := sshClient.UploadScript(file)
		out, err = scriptCmd.Execute()
		if err != nil {
			log.ErrorLn(strings.Trim(string(out), "\n"))
			return fmt.Errorf(
				"could not execute a script to get server hostname: %w",
				err,
			)
		}
		hostname := strings.Trim(string(out), "\n")
		log.DebugF("Server: %s hostname: %s\n", sshClient.Settings.Host(), hostname)
		if _, ok := serverHostnames[hostname]; ok {
			log.ErrorF("Server with hostname %s already exist!\n", hostname)
			serversWithError = append(serversWithError, sshClient.Settings.Host())
			return nil
		}

		serverHostnames[hostname] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}

	if len(serversWithError) > 0 {
		return fmt.Errorf(
			"please set unique hostname on the servers %s and try again",
			strings.Join(serversWithError, ","),
		)
	}

	return nil
}
