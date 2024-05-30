// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import "github.com/deckhouse/deckhouse/dhctl/pkg/log"

const (
	checkPortsScriptPath     = candiBashibleDir + "/preflight/check_ports.sh"
	checkLocalhostScriptPath = candiBashibleDir + "/preflight/check_localhost.sh"
	preflightScriptDirPath   = candiBashibleDir + "/preflight/"
)

func RenderAndSavePreflightCheckPortsScript() (string, error) {
	log.DebugLn("Start render check ports script")

	return RenderAndSaveTemplate("check_ports.sh", checkPortsScriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckLocalhostScript() (string, error) {
	log.DebugLn("Start render check localhost script")

	return RenderAndSaveTemplate(
		"check_localhost.sh",
		checkLocalhostScriptPath,
		map[string]interface{}{},
	)
}

func RenderAndSavePreflightCheckScript(
	filename string,
	params map[string]interface{},
) (string, error) {
	log.DebugLn("Start render check localhost script")

	return RenderAndSaveTemplate(
		filename,
		preflightScriptDirPath+filename,
		params,
	)
}
