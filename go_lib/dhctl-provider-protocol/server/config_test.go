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
	"context"
	"flag"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/client"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

func TestConfigGetterFromFlagsReadsWhatArgsWrites(t *testing.T) {
	args := server.ServeArgs("unix", "/tmp/v.sock")

	if args[0] != server.ServeCommand {
		t.Fatalf("Args() starts with %q, want %q", args[0], server.ServeCommand)
	}

	fs := flag.NewFlagSet(server.ServeCommand, flag.ContinueOnError)
	endpoint := server.ConfigGetterFromFlags(fs)

	if err := fs.Parse(args[1:]); err != nil {
		t.Fatalf("Parse(%q) = %v", args[1:], err)
	}

	if got := endpoint(); got.Network != "unix" || got.Address != "/tmp/v.sock" {
		t.Errorf("parsed %q %q, want unix /tmp/v.sock", got.Network, got.Address)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  server.Config
		wantErr string
	}{
		{
			name: "accepts a network, an address and a logger",
			config: server.Config{
				Network: "unix",
				Address: "/tmp/v.sock",
				Logger:  slog.Default(),
			},
		},
		{
			name: "rejects a missing network",
			config: server.Config{
				Address: "/tmp/v.sock",
				Logger:  slog.Default(),
			},
			wantErr: "network is required",
		},
		{
			// The caller allocates a fresh short path per run; a default would put
			// the socket at a world-writable well-known path.
			name: "rejects a missing address",
			config: server.Config{
				Network: "unix",
				Logger:  slog.Default(),
			},
			wantErr: "address is required",
		},
		{
			name: "rejects a missing logger",
			config: server.Config{
				Network: "unix",
				Address: "/tmp/v.sock",
			},
			wantErr: "logger is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()

			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Validate() = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     server.Config
		other    server.Config
		want     server.Config
		wantOpts int
	}{
		{
			name: "keeps the base when the other is empty",
			base: server.Config{Network: "unix", Address: "/tmp/base.sock"},
			want: server.Config{Network: "unix", Address: "/tmp/base.sock"},
		},
		{
			name:  "takes what the other sets",
			base:  server.Config{Network: "unix", Address: "/tmp/base.sock"},
			other: server.Config{Network: "tcp", Address: "127.0.0.1:0"},
			want:  server.Config{Network: "tcp", Address: "127.0.0.1:0"},
		},
		{
			name:     "adds the caller's options to the protocol's",
			base:     server.NewConfig(),
			other:    server.Config{Address: "127.0.0.1:0", GRPCOptions: []grpc.ServerOption{grpc.ConnectionTimeout(0)}},
			want:     server.Config{Network: server.DefaultNetwork, Address: "127.0.0.1:0"},
			wantOpts: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.base.Merge(test.other)

			if got.Network != test.want.Network || got.Address != test.want.Address {
				t.Errorf("Merge() = %+v, want %+v", got, test.want)
			}

			if test.wantOpts != 0 && len(got.GRPCOptions) != test.wantOpts {
				t.Errorf("GRPCOptions = %d, want %d", len(got.GRPCOptions), test.wantOpts)
			}
		})
	}
}

// The protocol mandates 8 MiB in each direction, and a caller passing an option of its
// own must not silently drop back to gRPC's own 4 MiB default.
func TestMergeKeepsTheProtocolMessageSize(t *testing.T) {
	const payload = 5 << 20

	running, err := server.Start(
		server.Config{
			Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
			GRPCOptions: []grpc.ServerOption{grpc.ConnectionTimeout(time.Minute)},
		},
		server.NewValidateService(validatorFunc(echoesBack)),
	)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	t.Cleanup(func() {
		if err := running.Stop(); err != nil {
			t.Errorf("Stop() = %v, want nil", err)
		}
	})

	conn, err := grpc.NewClient(running.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	validator := client.NewValidateClient(conn, client.Config{
		GRPCOptions: []grpc.CallOption{grpc.WaitForReady(true)},
	})

	input := bootstrapInput()
	input.ClusterPrefix = strings.Repeat("x", payload)

	resp, err := validator.Validate(context.Background(), input)
	if err != nil {
		t.Fatalf("Validate() = %v, want a %d byte payload to pass in both directions", err, payload)
	}

	if got := len(resp.GetErrors()[0].GetMessage()); got != payload {
		t.Errorf("response message = %d bytes, want %d", got, payload)
	}
}

// echoesBack sends the payload it was given back as a violation, so one call exercises
// the size limit in both directions.
func echoesBack(_ context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	return &validatev1.ValidateResponse{
		Errors: []*validatev1.ViolationResponse{{Message: input.ClusterPrefix}},
	}, nil
}

func TestListeningLineRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		network string
		address string
		wantOK  bool
	}{
		{
			name:    "reads back what a validator announced",
			line:    server.ListeningLine("tcp", "127.0.0.1:8080"),
			network: "tcp",
			address: "127.0.0.1:8080",
			wantOK:  true,
		},
		{
			// An IPv6 address carries brackets, and nothing about them ends the
			// endpoint: the address runs to the first space or quote.
			name:    "an IPv6 address keeps its brackets",
			line:    server.ListeningLine("tcp", "[::1]:43111"),
			network: "tcp",
			address: "[::1]:43111",
			wantOK:  true,
		},
		{
			// A socket path may carry spaces; only a quote or the line ends it.
			name:    "a socket path keeps its spaces",
			line:    server.ListeningLine("unix", "/tmp/dir with space/v.sock"),
			network: "unix",
			address: "/tmp/dir with space/v.sock",
			wantOK:  true,
		},
		{
			name:    "a socket path is an address like any other",
			line:    server.ListeningLine("unix", "/tmp/v.sock"),
			network: "unix",
			address: "/tmp/v.sock",
			wantOK:  true,
		},
		{
			name: "anything else the validator prints is not an announcement",
			line: `{"level":"info","msg":"Serve validator"}`,
		},
		{
			name: "the prefix alone announces nothing",
			line: server.ListeningPrefix,
		},
		{
			name: "half a line announces nothing",
			line: server.ListeningPrefix + "network: tcp",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			network, address, ok := server.ParseListeningLine(test.line)

			if ok != test.wantOK {
				t.Fatalf("ParseListeningLine(%q) ok = %v, want %v", test.line, ok, test.wantOK)
			}

			if network != test.network || address != test.address {
				t.Errorf("ParseListeningLine(%q) = %q %q, want %q %q",
					test.line, network, address, test.network, test.address)
			}
		})
	}
}
