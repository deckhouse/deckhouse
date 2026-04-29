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

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"
	"github.com/deckhouse/lib-dhctl/pkg/log"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type TunnelParams struct {
	DirectoryConfig *directoryconfig.DirectoryConfig
	LoggerProvider  log.LoggerProvider
	SSHClient       libcon.SSHClient
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

	logger := params.LoggerProvider()
	tunnel := newTunnel(params.DirectoryConfig, params.SSHClient)

	logger.DebugF("Up bundle registry tunnel...")
	if err := tunnel.start(ctx); err != nil {
		return nil, err
	}

	return func() {
		logger.DebugF("Stopping bundle registry tunnel...")
		tunnel.stop()
		logger.DebugF("Bundle registry tunnel: stopped")
	}, nil
}

func newTunnel(dc *directoryconfig.DirectoryConfig, sshCl libcon.SSHClient) *Tunnel {
	return &Tunnel{
		dc:      dc,
		sshCl:   sshCl,
		scheme:  constant.BundleScheme,
		address: constant.BundleAddress,
		port:    constant.BundlePort,
	}
}

type Tunnel struct {
	dc      *directoryconfig.DirectoryConfig
	sshCl   libcon.SSHClient
	scheme  constant.SchemeType
	address string
	port    string

	tunnel libcon.ReverseTunnel
}

func (t *Tunnel) start(ctx context.Context) error {
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

	checker := utils.NewRunScriptReverseTunnelChecker(t.sshCl, checkingScript)
	killer := utils.NewRunScriptReverseTunnelKiller(t.sshCl, killScript)

	// SSH reverse tunnel format: remoteHost:remotePort:localHost:localPort
	addr := fmt.Sprintf("%s:%s:%s:%s", t.address, t.port, t.address, t.port)

	tun := t.sshCl.ReverseTunnel(addr)
	if err = tun.Up(); err != nil {
		return err
	}

	tun.StartHealthMonitor(ctx, checker, killer)
	t.tunnel = tun
	return nil
}

func (t *Tunnel) stop() {
	if t.tunnel == nil {
		return
	}

	t.tunnel.Stop()
	t.tunnel = nil
}
