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

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var (
	checkPortsScriptPath              = candiBashibleDir + "/preflight/check_ports.sh.tpl"
	checkLocalhostScriptPath          = candiBashibleDir + "/preflight/check_localhost.sh.tpl"
	checkProxyRevTunnelOpenScriptPath = candiBashibleDir + "/preflight/check_reverse_tunnel_open.sh.tpl"
	killReverseTunnelPath             = candiBashibleDir + "/preflight/kill_reverse_tunnel.sh.tpl"
	checkDeckhouseUserScriptPath      = candiBashibleDir + "/preflight/check_deckhouse_user.sh.tpl"
	preflightScriptDirPath            = candiBashibleDir + "/preflight/"
)

func RenderAndSavePreflightCheckPortsScript(dc map[string]string) (string, error) {
	log.DebugLn("Start render check ports script")

	_, err := os.Stat(checkPortsScriptPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		checkPortsScriptPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight", "check_ports.sh.tpl")
	}

	return RenderAndSaveTemplate("check_ports.sh", checkPortsScriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckDeckhouseUserScript(dc map[string]string) (string, error) {
	log.DebugLn("Start render check user script")

	_, err := os.Stat(checkDeckhouseUserScriptPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		checkDeckhouseUserScriptPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight", "check_deckhouse_user.sh.tpl")
	}

	return RenderAndSaveTemplate("check_deckhouse_user.sh", checkDeckhouseUserScriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckLocalhostScript(dc map[string]string) (string, error) {
	log.DebugLn("Start render check localhost script")

	_, err := os.Stat(checkLocalhostScriptPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		checkLocalhostScriptPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight", "check_localhost.sh.tpl")
	}

	return RenderAndSaveTemplate(
		"check_localhost.sh",
		checkLocalhostScriptPath,
		map[string]interface{}{},
	)
}

func RenderAndSavePreflightReverseTunnelOpenScript(url string, dc map[string]string) (string, error) {
	log.DebugLn("Start render proxy reverse tunnel open script")

	_, err := os.Stat(checkProxyRevTunnelOpenScriptPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		checkProxyRevTunnelOpenScriptPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight", "check_reverse_tunnel_open.sh.tpl")
	}

	return RenderAndSaveTemplate(
		"check_reverse_tunnel_open.sh",
		checkProxyRevTunnelOpenScriptPath,
		map[string]interface{}{
			"url": url,
		},
	)
}

func RenderAndSaveKillReverseTunnelScript(host, port string, dc map[string]string) (string, error) {
	log.DebugLn("Start render kill reverse tunnel script")

	_, err := os.Stat(killReverseTunnelPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		killReverseTunnelPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight", "kill_reverse_tunnel.sh.tpl")
	}

	return RenderAndSaveTemplate(
		"kill_reverse_tunnel.sh",
		killReverseTunnelPath,
		map[string]interface{}{
			"host": host,
			"port": port,
		},
	)
}

func RenderAndSavePreflightCheckScript(
	filename string,
	params map[string]interface{},
	dc map[string]string,
) (string, error) {
	log.DebugLn("Start render check localhost script")

	_, err := os.Stat(preflightScriptDirPath)
	if err != nil {
		// fallback to /tmp
		downloadDir, ok := dc["downloadDir"]
		if !ok {
			return "", fmt.Errorf("could not get value of downloadDir from map %-v", dc)
		}
		preflightScriptDirPath = filepath.Join(downloadDir, "deckhouse", "candi", "bashible", "preflight")
	}

	return RenderAndSaveTemplate(
		filename,
		filepath.Join(preflightScriptDirPath, filename),
		params,
	)
}
