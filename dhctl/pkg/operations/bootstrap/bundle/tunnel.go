// Copyright 2026 Flant JSC
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

package bundle

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/deckhouse/lib-dhctl/pkg/log"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type TunnelParams struct {
	DirectoryConfig *directoryconfig.DirectoryConfig
	LoggerProvider  log.LoggerProvider
	SSHClient       node.SSHClient
}

func (params TunnelParams) Validate() error {
	if params.DirectoryConfig == nil {
		return fmt.Errorf("directory config is required")
	}

	if params.LoggerProvider == nil {
		return fmt.Errorf("logger provider is required")
	}

	if params.SSHClient == nil {
		return fmt.Errorf("ssh client is required")
	}

	return nil
}

type StopTunnel func()

func StartTunnel(ctx context.Context, params TunnelParams) (StopTunnel, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	tunnel := &Tunnel{
		dc:             params.DirectoryConfig,
		loggerProvider: params.LoggerProvider,
		sshCl:          params.SSHClient,

		scheme:  constant.BundleScheme,
		address: constant.BundleAddress,
		port:    constant.BundlePort,
	}

	if err := tunnel.start(ctx); err != nil {
		return nil, err
	}

	return tunnel.Stop, nil
}

type Tunnel struct {
	scheme  constant.SchemeType
	address string
	port    string

	dc             *directoryconfig.DirectoryConfig
	sshCl          node.SSHClient
	loggerProvider log.LoggerProvider

	tunnel node.ReverseTunnel
}

func (t *Tunnel) start(ctx context.Context) error {
	t.debug("Up bundle registry tunnel...")

	preflightURL := fmt.Sprintf(
		"%s://%s/healthz",
		strings.ToLower(string(t.scheme)),
		net.JoinHostPort(t.address, t.port),
	)

	checkingScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(preflightURL, t.dc)
	if err != nil {
		return fmt.Errorf("cannot render reverse tunnel checking script: %w", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(t.address, t.port, t.dc)
	if err != nil {
		return fmt.Errorf("cannot render kill reverse tunnel script: %w", err)
	}

	checker := ssh.NewRunScriptReverseTunnelChecker(t.sshCl, checkingScript)
	killer := ssh.NewRunScriptReverseTunnelKiller(t.sshCl, killScript)

	addr := fmt.Sprintf("%s:%s:%s:%s", t.address, t.port, t.address, t.port)

	tun := t.sshCl.ReverseTunnel(addr)
	if err = tun.Up(); err != nil {
		return err
	}

	tun.StartHealthMonitor(ctx, checker, killer)
	t.tunnel = tun
	return nil
}

func (t *Tunnel) Stop() {
	t.debug("Stopping bundle registry tunnel...")
	if t.tunnel == nil {
		t.debug("Bundle registry tunnel: skip stop because not initialized")
		return
	}

	t.tunnel.Stop()
	t.tunnel = nil
	t.debug("Bundle registry tunnel: stopped")
}

func (t *Tunnel) debug(f string, args ...any) {
	t.loggerProvider().DebugF(f, args...)
}
