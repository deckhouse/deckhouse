/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package revocationfeed

import (
	"context"
	"fmt"
	"net"
	"time"
)

// A peer that is gone without closing the connection - a node or a network that went away - is
// noticed by the kernel only when TCP gives up retransmitting: about 15 minutes with the Linux
// defaults (net.ipv4.tcp_retries2). The heartbeats a stream writes every 10 seconds fit in the
// send buffer for all that time, so the write deadline never fires, and the stream of a client
// that is gone stays in the subscribers gauge. The options below make the kernel drop such a
// connection within userTimeout (listen_linux.go); the next write fails and the stream closes.
const (
	// keepAliveIdle, keepAliveInterval and keepAliveCount probe a connection with nothing to send:
	// a peer that is gone is found within 30 seconds.
	keepAliveIdle     = 15 * time.Second
	keepAliveInterval = 5 * time.Second
	keepAliveCount    = 3
)

// listen listens on addr; every connection it accepts has the options above.
func listen(ctx context.Context, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     keepAliveIdle,
			Interval: keepAliveInterval,
			Count:    keepAliveCount,
		},
		// Set on the listening socket: the connections it accepts inherit it.
		Control: setUserTimeout,
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return ln, nil
}
