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

package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers"
	v1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/socket"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/tcp"
	d8requirements "github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

// buildDebugServers creates both introspection servers with the route tree each
// of them serves. Nothing is bound here: startDebugServers binds the listeners
// once the runtime is actually running.
//
// The socket gets the private tree, which exposes package values, rendered
// manifests and hook snapshots; the TCP listener gets the public tree only.
func (r *Runtime) buildDebugServers() {
	deps := v1.Deps{
		Packages:     r,
		Queues:       r,
		Scheduler:    r.scheduler,
		Requirements: d8requirements.DumpValues,
	}

	r.socketServer = socket.New(debugSocketPath, handlers.NewPrivateRouter(deps), r.logger)
	r.tcpServer = tcp.New(debugTCPAddress, debugTCPPort, handlers.NewPublicRouter(deps), r.logger)
}

// startDebugServers binds the socket and the loopback TCP listener.
func (r *Runtime) startDebugServers() error {
	if err := r.socketServer.Start(); err != nil {
		return fmt.Errorf("start socket server: %w", err)
	}

	if err := r.tcpServer.Start(); err != nil {
		return fmt.Errorf("start tcp server: %w", err)
	}

	return nil
}

// stopDebugServers closes both listeners; closing the socket unlinks its file.
func (r *Runtime) stopDebugServers(ctx context.Context) error {
	return errors.Join(r.socketServer.Shutdown(ctx), r.tcpServer.Shutdown(ctx))
}
