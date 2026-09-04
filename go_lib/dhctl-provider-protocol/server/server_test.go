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

package server_test

import (
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

// The caller finds the endpoint by reading it out of the process output, so the line
// goes to stdout rather than through the validator's logger, where a log level could
// hide it.
func TestStartAnnouncesTheEndpointOnStdout(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() = %v", err)
	}

	stdout := os.Stdout
	os.Stdout = write

	// A logger that drops everything: the announcement must not depend on it.
	running, startErr := server.Start(server.Config{
		Logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
	})

	os.Stdout = stdout

	if err := write.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if startErr != nil {
		t.Fatalf("Start() = %v", startErr)
	}

	t.Cleanup(func() {
		if err := running.Stop(); err != nil {
			t.Errorf("Stop() = %v, want nil", err)
		}
	})

	printed, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}

	network, address, ok := server.ParseListeningLine(string(printed))
	if !ok {
		t.Fatalf("printed %q, want an announcement", printed)
	}

	if network != running.Addr().Network() || address != running.Addr().String() {
		t.Errorf("announced %s %s, want %s %s",
			network, address, running.Addr().Network(), running.Addr().String())
	}
}

func TestStart(t *testing.T) {
	tests := []struct {
		name        string
		network     string // empty means the protocol's default, tcp
		services    int
		omitAddress bool
	}{
		{
			// A caller that names no address gets loopback on a port the kernel
			// picks, and reads it back from Addr.
			name:        "falls back to the default address",
			omitAddress: true,
		},
		{
			// A server with no action serves nothing but still listens: a caller
			// learns about a missing action from Unimplemented.
			name: "starts without services",
		},
		{
			name:     "registers every service it is given",
			services: 2,
		},
		{
			// dhctl dials loopback, but the protocol takes a unix socket too and a
			// caller may switch to it.
			name:    "listens on a unix socket too",
			network: "unix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			network := test.network
			if network == "" {
				network = server.DefaultNetwork
			}

			address := ""

			switch {
			case test.omitAddress:
			case network == "unix":
				address = socketPath(t)
			default:
				address = loopbackAddress(t)
			}

			registrars := make([]*countingService, test.services)
			services := make([]server.Service, 0, test.services)

			for i := range registrars {
				registrars[i] = &countingService{}
				services = append(services, registrars[i])
			}

			running, err := server.Start(server.Config{Network: test.network, Address: address}, services...)
			if err != nil {
				t.Fatalf("Start() = %v", err)
			}

			for i, registrar := range registrars {
				if got := registrar.calls(); got != 1 {
					t.Errorf("service %d registered %d times, want 1", i, got)
				}
			}

			// Addr is the whole point of the default address: the port is the
			// kernel's, so this is the only way to learn where to dial.
			addr := running.Addr()
			if addr == nil {
				t.Fatal("Addr() = nil, want the listening address")
			}

			if test.omitAddress && !strings.HasPrefix(addr.String(), "127.0.0.1:") {
				t.Errorf("Addr() = %s, want a loopback address", addr)
			}

			// The listener is up by the time Start returns, so a caller may connect
			// right away.
			conn, err := net.Dial(addr.Network(), addr.String())
			if err != nil {
				t.Fatalf("Dial() = %v", err)
			}

			_ = conn.Close()

			if err := running.Stop(); err != nil {
				t.Fatalf("Stop() = %v", err)
			}

			// Stop is idempotent: a second call repeats the first one's result
			// instead of blocking on a drained channel.
			if err := running.Stop(); err != nil {
				t.Errorf("second Stop() = %v, want nil", err)
			}
		})
	}
}

// countingService stands in for an action: the transport only has to register what
// it is given.
type countingService struct {
	mu         sync.Mutex
	registered int
}

func (s *countingService) Register(grpc.ServiceRegistrar) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registered++
}

func (s *countingService) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.registered
}

// loopbackAddress reserves a loopback port and hands it back, the way the host picks
// one before spawning a validator. The window between the close and the server's bind
// is the same race the host lives with.
func loopbackAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen(server.DefaultNetwork, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}

	address := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	return address
}

// socketPath builds a path outside t.TempDir, which spells the test's name into it
// and on its own can exceed sun_path (104 bytes on darwin, 108 on Linux).
func socketPath(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "d8v")
	if err != nil {
		t.Fatalf("MkdirTemp() = %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return filepath.Join(dir, "v.sock")
}
