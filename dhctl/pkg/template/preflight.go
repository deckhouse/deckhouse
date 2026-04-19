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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
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

func RenderAndSavePreflightCheckPortsScript(dc *directoryconfig.DirectoryConfig) (string, error) {
	log.DebugLn("Start render check ports script")

	if _, err := os.Stat(checkPortsScriptPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		checkPortsScriptPath = getPreflightPath(dc.DownloadDir, "check_ports.sh.tpl")
	}

	return RenderAndSaveTemplate("check_ports.sh", checkPortsScriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckDeckhouseUserScript(dc *directoryconfig.DirectoryConfig) (string, error) {
	log.DebugLn("Start render check user script")

	if _, err := os.Stat(checkDeckhouseUserScriptPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		checkDeckhouseUserScriptPath = getPreflightPath(dc.DownloadDir, "check_deckhouse_user.sh.tpl")
	}

	return RenderAndSaveTemplate("check_deckhouse_user.sh", checkDeckhouseUserScriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckLocalhostScript(dc *directoryconfig.DirectoryConfig) (string, error) {
	log.DebugLn("Start render check localhost script")

	if _, err := os.Stat(checkLocalhostScriptPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		checkLocalhostScriptPath = getPreflightPath(dc.DownloadDir, "check_localhost.sh.tpl")
	}

	return RenderAndSaveTemplate(
		"check_localhost.sh",
		checkLocalhostScriptPath,
		map[string]interface{}{},
	)
}

func RenderAndSavePreflightReverseTunnelOpenScript(url string, dc *directoryconfig.DirectoryConfig) (string, error) {
	log.DebugLn("Start render proxy reverse tunnel open script")

	if _, err := os.Stat(checkProxyRevTunnelOpenScriptPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		checkProxyRevTunnelOpenScriptPath = getPreflightPath(dc.DownloadDir, "check_reverse_tunnel_open.sh.tpl")
	}

	return RenderAndSaveTemplate(
		"check_reverse_tunnel_open.sh",
		checkProxyRevTunnelOpenScriptPath,
		map[string]interface{}{
			"url": url,
		},
	)
}

func RenderAndSaveKillReverseTunnelScript(host, port string, dc *directoryconfig.DirectoryConfig) (string, error) {
	log.DebugLn("Start render kill reverse tunnel script")

	if _, err := os.Stat(killReverseTunnelPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		killReverseTunnelPath = getPreflightPath(dc.DownloadDir, "kill_reverse_tunnel.sh.tpl")
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
	dc *directoryconfig.DirectoryConfig,
) (string, error) {
	log.DebugLn("Start render check localhost script")

	if _, err := os.Stat(preflightScriptDirPath); err != nil {
		if dc == nil {
			return "", fmt.Errorf("could not get value of dc.DownloadDir")
		}
		preflightScriptDirPath = getPreflightPath(dc.DownloadDir, "")
	}

	return RenderAndSaveTemplate(
		filename,
		filepath.Join(preflightScriptDirPath, filename),
		params,
	)
}

func getPreflightPath(rootDir, dest string) string {
	path := filepath.Join(rootDir, "deckhouse", "candi", "bashible", "preflight")
	if dest != "" {
		path = filepath.Join(path, dest)
	}

	return path
}
