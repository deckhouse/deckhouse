// Copyright 2023 Flant JSC
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
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	defaultTunnelLocalPort       = 27322
	defaultTunnelRemotePort      = 27322
	localhost                    = "127.0.0.1"
	reverseTunnelScriptUploadDir = "/tmp"
	httpPath                     = "/healthz"
)

var ErrAuthSSHFailed = errors.New("authentication failed")

func (pc *Checker) CheckSSHTunnel(ctx context.Context) error {
	if app.PreflightSkipSSHForward {
		log.InfoLn("SSH forward preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("SSH forward preflight check was skipped (local run)")
		return nil
	}

	log.DebugF(
		"Checking ssh tunnel with remote port %d and local port %d\n",
		defaultTunnelRemotePort,
		defaultTunnelLocalPort,
	)

	remotePortStr := strconv.Itoa(defaultTunnelRemotePort)

	checkingScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(healthUrl(defaultTunnelRemotePort))
	if err != nil {
		return fmt.Errorf("Cannot render reverse tunnel checking script: %v", err)
	}

	killScript, err := template.RenderAndSaveKillReverseTunnelScript(localhost, remotePortStr)
	if err != nil {
		return fmt.Errorf("Cannot render kill reverse tunnel script: %v", err)
	}

	shutdownServer, err := startHttpServer(ctx, defaultTunnelLocalPort)
	if err != nil {
		return err
	}

	defer shutdownServer()

	local := net.JoinHostPort(localhost, strconv.Itoa(defaultTunnelLocalPort))
	remote := net.JoinHostPort(localhost, remotePortStr)
	addr := strings.Join([]string{local, remote}, ":")

	sshCl := wrapper.Client()

	tun := sshCl.ReverseTunnel(addr)
	err = tun.Up()
	if err != nil {
		return getTunnelPreflightCheckFailedError(err, "")
	}

	defer func() {
		tun.Stop()
	}()

	log.DebugLn("Performing tunnel health check")

	checkStdout, err := ssh.NewRunScriptReverseTunnelChecker(sshCl, checkingScript).
		SetUploadDirAndCleanup(reverseTunnelScriptUploadDir).
		CheckTunnel(ctx)

	// yes first kill tunnel, next check error after check
	killStdout, killErr := ssh.NewRunScriptReverseTunnelKiller(sshCl, killScript).
		SetUploadDirAndCleanup(reverseTunnelScriptUploadDir).
		KillTunnel(ctx)
	if killErr != nil {
		killErr = fmt.Errorf("Error killing ssh tunnel on remote port %d: %v", defaultTunnelRemotePort, killErr)
		log.DebugF("%v\nstdout: %s\n", killErr, killStdout)
		return killErr
	}

	if err != nil {
		return getTunnelPreflightCheckFailedError(err, checkStdout)
	}

	log.DebugLn("Tunnel health check passed")

	return nil
}

func (pc *Checker) CheckSSHCredential(ctx context.Context) error {
	if app.PreflightSkipSSHCredentialsCheck {
		log.InfoLn("SSH credentials preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("SSH credentials preflight check was skipped (local run)")
		return nil
	}

	sshCheck := wrapper.Client().Check()
	err := sshCheck.CheckAvailability(ctx)
	if err != nil {
		return fmt.Errorf(
			"ssh %w. Please check ssh credential and try again. Error: %w",
			ErrAuthSSHFailed, err,
		)
	}
	return nil
}

func (pc *Checker) CheckSingleSSHHostForStatic(_ context.Context) error {
	if app.PreflightSkipOneSSHHost {
		log.InfoLn("Only one --ssh-host parameter used preflight check was skipped (via skip flag)")
		return nil
	}

	wrapper, ok := pc.nodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		log.InfoLn("Only one --ssh-host parameter used preflight check was skipped (local run)")
		return nil
	}

	if len(wrapper.Client().Session().AvailableHosts()) > 1 {
		return fmt.Errorf(
			"during the bootstrap of the first static master node, only one --ssh-host parameter is allowed",
		)
	}
	return nil
}

type shutdownServerFunc func()

func startHttpServer(ctx context.Context, port int) (shutdownServerFunc, error) {
	mux := http.NewServeMux()

	// Register handlers for specific paths
	mux.HandleFunc(httpPath, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK\n")
	})

	address := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		log.DebugF("Starting HTTP server for tunnel preflight check on %s\n", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.InfoF("Error starting HTTP server for tunnel preflight check on %s: %v\n", address, err)
		}
	}()

	shutdownServer := func() {
		err := server.Shutdown(ctx)
		if err != nil {
			log.WarnF("Error shutting down server for checking ssh tunnel: %v\n", err)
			return
		}

		log.DebugLn("Server for checking ssh tunnel stopped")
	}

	url := healthUrl(defaultTunnelLocalPort)

	client := &http.Client{}

	err := retry.NewSilentLoop("Check HTTP server running for tunnel preflight check", 5, 1*time.Millisecond).RunContext(ctx, func() error {
		cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel() // Ensure the context is canceled to release resources

		// Create a new HTTP GET request with the context
		req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
		if err != nil {
			log.DebugF("Error making GET request for checking preflight tunnel: %v\n", err)
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			log.DebugF("Error do GET request for checking preflight tunnel: %v\n", err)
			return err
		}

		if err := resp.Body.Close(); err != nil {
			log.DebugF("Error closing response body for checking preflight tunnel: %v\n", err)
		}

		return nil
	})

	if err != nil {
		log.ErrorF("Error starting HTTP server for tunnel preflight check on %s: %v\n", address, err)

		shutdownServer()

		return nil, err
	}

	return shutdownServer, nil
}

func healthUrl(port int) string {
	return fmt.Sprintf("http://%s:%d%s", localhost, port, httpPath)
}

func getTunnelPreflightCheckFailedError(err error, stdout string) error {
	if stdout != "" {
		log.DebugF("Error checking ssh tunnel: %v\nstdout: %s\n", err, stdout)
	}

	return fmt.Errorf(`Cannot establish working tunnel to control-plane host: %w
Please check connectivity to control-plane host and that the sshd config parameters 'AllowTcpForwarding' is set to 'yes' and 'DisableForwarding' is set to 'no' on the control-plane node`, err)
}
