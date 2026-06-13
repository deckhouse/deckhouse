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
	"context"
	"path/filepath"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

var (
	checkPortsScriptPath              = filepath.Join("preflight", "check_ports.sh.tpl")
	checkLocalhostScriptPath          = filepath.Join("preflight", "check_localhost.sh.tpl")
	checkProxyRevTunnelOpenScriptPath = filepath.Join("preflight", "check_reverse_tunnel_open.sh.tpl")
	killReverseTunnelPath             = filepath.Join("preflight", "kill_reverse_tunnel.sh.tpl")
	checkDeckhouseUserScriptPath      = filepath.Join("preflight", "check_deckhouse_user.sh.tpl")
	preflightScriptDirPath            = "preflight"
)

func RenderAndSavePreflightCheckPortsScript(ctx context.Context, globalOptions *options.GlobalOptions) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering check ports script")
	scriptPath := filepath.Join(globalOptions.CandiDir, "bashible", checkPortsScriptPath)

	return RenderAndSaveTemplate(ctx, "check_ports.sh", scriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckDeckhouseUserScript(ctx context.Context, globalOptions *options.GlobalOptions) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering check user script")
	scriptPath := filepath.Join(globalOptions.CandiDir, "bashible", checkDeckhouseUserScriptPath)

	return RenderAndSaveTemplate(ctx, "check_deckhouse_user.sh", scriptPath, map[string]interface{}{})
}

func RenderAndSavePreflightCheckLocalhostScript(ctx context.Context, globalOptions *options.GlobalOptions) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering check localhost script")
	scriptPath := filepath.Join(globalOptions.CandiDir, "bashible", checkLocalhostScriptPath)

	return RenderAndSaveTemplate(
		ctx,
		"check_localhost.sh",
		scriptPath,
		map[string]interface{}{},
	)
}

func RenderAndSavePreflightReverseTunnelOpenScript(ctx context.Context, url string, globalOptions *options.GlobalOptions) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering proxy reverse tunnel open script")
	scriptPath := filepath.Join(globalOptions.CandiDir, "bashible", checkProxyRevTunnelOpenScriptPath)

	return RenderAndSaveTemplate(
		ctx,
		"check_reverse_tunnel_open.sh",
		scriptPath,
		map[string]interface{}{
			"url": url,
		},
	)
}

func RenderAndSaveKillReverseTunnelScript(ctx context.Context, host, port string, globalOptions *options.GlobalOptions) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering kill reverse tunnel script")
	scriptPath := filepath.Join(globalOptions.CandiDir, "bashible", killReverseTunnelPath)

	return RenderAndSaveTemplate(
		ctx,
		"kill_reverse_tunnel.sh",
		scriptPath,
		map[string]interface{}{
			"host": host,
			"port": port,
		},
	)
}

func RenderAndSavePreflightCheckScript(
	ctx context.Context,
	filename string,
	params map[string]interface{},
	globalOptions *options.GlobalOptions,
) (string, error) {
	dhlog.FromContext(ctx).DebugContext(ctx, "Rendering check localhost script")
	path := filepath.Join(globalOptions.CandiDir, "bashible", preflightScriptDirPath)

	return RenderAndSaveTemplate(
		ctx,
		filename,
		filepath.Join(path, filename),
		params,
	)
}
