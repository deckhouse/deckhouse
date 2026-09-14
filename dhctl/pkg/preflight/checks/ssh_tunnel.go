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

package checks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type SSHTunnelCheck struct {
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	globalOptions          *options.GlobalOptions
}

const (
	defaultTunnelLocalPort  = 27322
	defaultTunnelRemotePort = 27322
	localhost               = "127.0.0.1"
	httpPath                = "/healthz"
)

const SSHTunnelCheckName preflight.CheckName = "static-ssh-tunnel"

func (SSHTunnelCheck) Description() string {
	return "the node can open a reverse tunnel back to the installer"
}

func (SSHTunnelCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SSHTunnelCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c SSHTunnelCheck) Run(ctx context.Context) (string, error) {
	nodeInterface, err := helper.GetNodeInterface(ctx, c.SSHProviderInitializer, c.SSHProviderInitializer.GetSettings())
	if err != nil {
		return "", err
	}
	wrapper, ok := nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return "", preflight.NotApplicable("dhctl was given no SSH host: there is no tunnel to open")
	}
	sshCl := wrapper.Client()
	host := hostLabelOfClient(sshCl)

	checkScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(ctx, healthURL(defaultTunnelRemotePort), c.globalOptions)
	if err != nil {
		return "", fmt.Errorf("render reverse tunnel script: %w", err)
	}
	killScript, err := template.RenderAndSaveKillReverseTunnelScript(ctx, localhost, strconv.Itoa(defaultTunnelRemotePort), c.globalOptions)
	if err != nil {
		return "", fmt.Errorf("render kill tunnel script: %w", err)
	}

	shutdown, err := startHTTPServer(ctx, defaultTunnelLocalPort)
	if err != nil {
		// This one is about this host, not the node: something local already holds the port.
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("port %d on the installer host", defaultTunnelLocalPort),
			Observed: classifyNetworkError(err),
			Expected: fmt.Sprintf("port %d free for the node to connect back to", defaultTunnelLocalPort),
			Fix:      fmt.Sprintf("stop whatever holds %d on this host (ss -lntp | grep %d)", defaultTunnelLocalPort, defaultTunnelLocalPort),
			Err:      err,
		})
	}
	defer shutdown()

	addr := strings.Join([]string{
		net.JoinHostPort(localhost, strconv.Itoa(defaultTunnelLocalPort)),
		net.JoinHostPort(localhost, strconv.Itoa(defaultTunnelRemotePort)),
	}, ":")

	tun := sshCl.ReverseTunnel(addr)
	if err := tun.Up(); err != nil {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("reverse ssh tunnel %d -> %s:%d", defaultTunnelLocalPort, host, defaultTunnelRemotePort),
			Observed: "the node refused to open the reverse forward",
			Expected: "sshd on the node to allow a remote port forward",
			Fix: "set AllowTcpForwarding yes and DisableForwarding no in sshd_config on the node; " +
				"GatewayPorts is not required, the forward binds to 127.0.0.1",
			Err: err,
		}
	}
	defer tun.Stop()

	// The forward is up; what is left is whether the node can actually use it. Deckhouse
	// bootstraps through this tunnel, so a forward that opens but carries nothing is the same
	// failure, later and harder to read.
	if _, err := utils.NewRunScriptReverseTunnelChecker(sshCl, checkScript).
		SetUploadDirAndCleanup("/tmp").
		CheckTunnel(ctx); err != nil {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s through the reverse tunnel", healthURL(defaultTunnelRemotePort), host),
			Observed: "the node opened the tunnel but could not reach back through it",
			Expected: "the node to reach the installer on the forwarded port",
			Fix: fmt.Sprintf("check that nothing on the node blocks 127.0.0.1:%d (a local firewall, SELinux), "+
				"and that sshd has AllowTcpForwarding yes", defaultTunnelRemotePort),
			Err: err,
		}
	}

	if _, err := utils.NewRunScriptReverseTunnelKiller(sshCl, killScript).
		SetUploadDirAndCleanup("/tmp").
		KillTunnel(ctx); err != nil {
		return "", fmt.Errorf("cannot close the test tunnel on port %d on %s: %w", defaultTunnelRemotePort, host, err)
	}

	return fmt.Sprintf("%s reached the installer back through a reverse tunnel on port %d", host, defaultTunnelRemotePort), nil
}

func healthURL(port int) string {
	return fmt.Sprintf("http://%s:%d%s", localhost, port, httpPath)
}

type shutdownServerFunc func()

func startHTTPServer(ctx context.Context, port int) (shutdownServerFunc, error) {
	mux := http.NewServeMux()
	mux.HandleFunc(httpPath, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "OK\n") })

	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("cannot start HTTP server for tunnel preflight check on %s: %w", address, err)
	}

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return
		}
	}()

	return func() { _ = server.Shutdown(ctx) }, nil
}

func SSHTunnel(sshProviderInitializer *providerinitializer.SSHProviderInitializer, globalOptions *options.GlobalOptions) preflight.Check {
	check := SSHTunnelCheck{SSHProviderInitializer: sshProviderInitializer, globalOptions: globalOptions}
	return preflight.Check{
		Name:        SSHTunnelCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}
