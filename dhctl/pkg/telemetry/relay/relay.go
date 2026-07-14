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

package relay

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"go.opentelemetry.io/otel/trace"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/utils"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

const (
	RelayPort    = "4318" // Common OTel HTTP port, but we might pick random or hardcode for bashible
	RelayAddress = "127.0.0.1"
)

type RelayParams struct {
	TracerName string
	Span       trace.Span
	Node       libcon.Interface
	Logger     *slog.Logger
	GlobalOpts *options.GlobalOptions
}

type Relay struct {
	params RelayParams
	server *Server
	tunnel libcon.ReverseTunnel
}

type stopFunc func()
type updateRelaySpan func(span trace.Span)

func InitRelay(ctx context.Context, params RelayParams) (stopFunc, updateRelaySpan, error) {
	nop := func() {}
	nopSpan := func(span trace.Span) {}

	if !telemetry.IsEnabled() {
		return nop, nopSpan, nil
	}

	wrapper, ok := params.Node.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nop, nopSpan, nil // Can't start tunnel without SSH
	}

	// Find free local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nop, nopSpan, fmt.Errorf("find free port for OTel relay: %w", err)
	}
	localPort := fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	r := &Relay{
		params: params,
		server: NewServer(params.Span, params.Logger, params.TracerName),
	}

	// Start local server
	if err := r.server.Start(ctx, "127.0.0.1:"+localPort); err != nil {
		return nop, nopSpan, fmt.Errorf("start OTel relay server: %w", err)
	}

	// Start reverse tunnel
	addr := fmt.Sprintf("%s:%s:127.0.0.1:%s", RelayAddress, RelayPort, localPort)
	tun := wrapper.Client().ReverseTunnel(addr)

	if err := tun.Up(); err != nil {
		_ = r.server.Stop(ctx)
		return nop, nopSpan, fmt.Errorf("start OTel relay reverse tunnel: %w", err)
	}

	// Create checker/killer for health monitor
	checkScript, err := template.RenderAndSavePreflightReverseTunnelOpenScript(
		ctx,
		fmt.Sprintf("http://%s:%s/healthz", RelayAddress, RelayPort),
		params.GlobalOpts,
	)
	if err == nil {
		killScript, err := template.RenderAndSaveKillReverseTunnelScript(ctx, RelayAddress, RelayPort, params.GlobalOpts)
		if err == nil {
			checker := utils.NewRunScriptReverseTunnelChecker(wrapper.Client(), checkScript)
			killer := utils.NewRunScriptReverseTunnelKiller(wrapper.Client(), killScript)
			tun.StartHealthMonitor(ctx, checker, killer)
		}
	}

	r.tunnel = tun

	return func() {
		if r.tunnel != nil {
			// lib-connection's ReverseTunnel.Stop() can deadlock on an internal
			// channel send when the health monitor has been flapping — e.g. the
			// reverse tunnel never became reachable, as happens when dhctl runs in
			// a local container bootstrapping a cloud master (the master can't
			// health-check the tunnel back). A stuck Stop() would block dhctl
			// teardown forever (tomb.WaitShutdown), so bound it with a timeout and
			// move on; the leaked goroutine dies with the process.
			done := make(chan struct{})
			go func() {
				r.tunnel.Stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
			}
		}
		if r.server != nil {
			_ = r.server.Stop(context.Background())
		}
	}, r.server.UpdateSpan, nil
}
