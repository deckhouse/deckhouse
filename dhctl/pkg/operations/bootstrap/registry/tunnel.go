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

package registry

import (
	"context"
	"fmt"
	"net"
	"strings"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"
	"github.com/deckhouse/lib-dhctl/pkg/log"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// TunnelParams holds dependencies required to establish the SSH reverse tunnel.
type TunnelParams struct {
	MetaConfig *config.MetaConfig
	Node       libcon.Interface
	Logger     log.Logger
	DirsConfig *directoryconfig.DirectoryConfig
}

func (params TunnelParams) Validate() error {
	if params.MetaConfig == nil {
		return fmt.Errorf("internal error: meta config is required")
	}

	if params.Node == nil {
		return fmt.Errorf("internal error: node client is required")
	}

	if params.Logger == nil {
		return fmt.Errorf("internal error: logger is required")
	}

	if params.DirsConfig == nil {
		return fmt.Errorf("internal error: directory config is required")
	}

	return nil
}

// InitTunnel starts an SSH reverse tunnel so the bootstrap target can reach the
// local OCI bundle registry when the registry mode is Local and not a standalone install.
// Returns a Close function to gracefully shut down the tunnel,
// or a no-op function if the tunnel was not started.
func InitTunnel(ctx context.Context, params TunnelParams) (StopTunnel, error) {
	nop := func() {}

	if err := params.Validate(); err != nil {
		return nop, err
	}

	if !params.MetaConfig.Registry.IsLocal() {
		return nop, nil
	}

	// Standalone (non-SSH) installs have no remote host to tunnel to.
	wrapper, ok := params.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nop, nil
	}

	logger := params.Logger
	logger.DebugF("Up bundle registry tunnel...")

	tunnel := newTunnel(params.DirsConfig, wrapper.Client())
	if err := tunnel.start(ctx); err != nil {
		return nop, fmt.Errorf("start bundle registry tunnel: %w", err)
	}

	return func() {
		logger.DebugF("Stopping bundle registry tunnel...")
		tunnel.stop()
		logger.DebugF("Bundle registry tunnel: stopped")
	}, nil
}

// newTunnel creates a Tunnel pre-configured with the bundle-specific scheme, address, and port.
func newTunnel(dc *directoryconfig.DirectoryConfig, sshCl libcon.SSHClient) *tunnel {
	return &tunnel{
		dc:      dc,
		sshCl:   sshCl,
		scheme:  constant.BundleScheme,
		address: constant.BundleAddress,
		port:    constant.BundlePort,
	}
}

// tunnel manages the SSH reverse tunnel lifecycle for the bundle registry.
type tunnel struct {
	dc      *directoryconfig.DirectoryConfig
	sshCl   libcon.SSHClient
	scheme  constant.SchemeType
	address string
	port    string

	tunnel libcon.ReverseTunnel
}

func (t *tunnel) start(ctx context.Context) error {
	preflightURL := fmt.Sprintf(
		"%s://%s/v2/",
		strings.ToLower(string(t.scheme)),
		net.JoinHostPort(t.address, t.port),
	)

	checkScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(preflightURL, t.dc)
	if err != nil {
		return fmt.Errorf("cannot render reverse tunnel checking script: %w", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(t.address, t.port, t.dc)
	if err != nil {
		return fmt.Errorf("cannot render kill reverse tunnel script: %w", err)
	}

	checker := utils.NewRunScriptReverseTunnelChecker(t.sshCl, checkScript)
	killer := utils.NewRunScriptReverseTunnelKiller(t.sshCl, killScript)

	// remoteBindAddress:remotePort:localHost:localPort — binds on the same host/port on both
	// ends so that the remote side can reach the local registry at the same address it expects.
	addr := fmt.Sprintf("%s:%s:%s:%s", t.address, t.port, t.address, t.port)

	tun := t.sshCl.ReverseTunnel(addr)
	if err = tun.Up(); err != nil {
		return err
	}

	tun.StartHealthMonitor(ctx, checker, killer)
	t.tunnel = tun
	return nil
}

func (t *tunnel) stop() {
	if t == nil {
		return
	}

	if t.tunnel != nil {
		t.tunnel.Stop()
		t.tunnel = nil
	}
}
