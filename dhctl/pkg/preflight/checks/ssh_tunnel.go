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

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type SSHTunnelCheck struct{ Node node.Interface }

const (
	defaultTunnelLocalPort  = 27322
	defaultTunnelRemotePort = 27322
	localhost               = "127.0.0.1"
	httpPath                = "/healthz"
)

const SSHTunnelCheckName preflight.CheckName = "static-ssh-tunnel"

func (SSHTunnelCheck) Description() string {
	return "ssh tunnel between installer and node is possible"
}

func (SSHTunnelCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (SSHTunnelCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.DefaultRetryPolicy
}

func (c SSHTunnelCheck) Run(ctx context.Context) error {
	wrapper, ok := c.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}

	checkScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(healthURL(defaultTunnelRemotePort))
	if err != nil {
		return fmt.Errorf("render reverse tunnel script: %w", err)
	}
	killScript, err := template.RenderAndSaveKillReverseTunnelScript(localhost, strconv.Itoa(defaultTunnelRemotePort))
	if err != nil {
		return fmt.Errorf("render kill tunnel script: %w", err)
	}

	shutdown, err := startHTTPServer(ctx, defaultTunnelLocalPort)
	if err != nil {
		return err
	}
	defer shutdown()

	sshCl := wrapper.Client()
	addr := strings.Join([]string{
		net.JoinHostPort(localhost, strconv.Itoa(defaultTunnelLocalPort)),
		net.JoinHostPort(localhost, strconv.Itoa(defaultTunnelRemotePort)),
	}, ":")

	tun := sshCl.ReverseTunnel(addr)
	if err := tun.Up(); err != nil {
		return fmt.Errorf("ssh tunnel setup failed: %w", err)
	}
	defer tun.Stop()

	if _, err := ssh.NewRunScriptReverseTunnelChecker(sshCl, checkScript).
		SetUploadDirAndCleanup("/tmp").
		CheckTunnel(ctx); err != nil {
		return fmt.Errorf("ssh tunnel health check failed: %w", err)
	}

	if _, err := ssh.NewRunScriptReverseTunnelKiller(sshCl, killScript).
		SetUploadDirAndCleanup("/tmp").
		KillTunnel(ctx); err != nil {
		return fmt.Errorf("error killing ssh tunnel on remote port %d: %v", defaultTunnelRemotePort, err)
	}

	return nil
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

func SSHTunnel(nodeInterface node.Interface) preflight.Check {
	check := SSHTunnelCheck{Node: nodeInterface}
	return preflight.Check{
		Name:        SSHTunnelCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
