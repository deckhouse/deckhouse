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
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/client"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  server.Config
		wantErr string
	}{
		{name: "accepts a network and an address", config: server.Config{Network: "unix", Address: "/tmp/v.sock"}},
		{name: "rejects a missing network", config: server.Config{Address: "/tmp/v.sock"}, wantErr: "network is required"},
		{
			// The caller allocates a fresh short path per run; a default would put
			// the socket at a world-writable well-known path.
			name:    "rejects a missing address",
			config:  server.Config{Network: "unix"},
			wantErr: "address is required",
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
		name  string
		base  server.Config
		other server.Config
		want  server.Config
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.base.Merge(test.other)

			if got.Network != test.want.Network || got.Address != test.want.Address {
				t.Errorf("Merge() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestStart(t *testing.T) {
	tests := []struct {
		name         string
		services     int
		omitAddress  bool
		wantStartErr bool
	}{
		{
			// The caller allocates the socket path; there is no default to fall
			// back on.
			name:         "refuses to start without an address",
			omitAddress:  true,
			wantStartErr: true,
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address := ""
			if !test.omitAddress {
				address = socketPath(t)
			}

			registrars := make([]*countingService, test.services)
			services := make([]server.Service, 0, test.services)

			for i := range registrars {
				registrars[i] = &countingService{}
				services = append(services, registrars[i])
			}

			running, err := server.Start(server.Config{Address: address}, services...)

			if test.wantStartErr {
				if err == nil {
					_ = running.Stop()
					t.Fatal("Start() = nil, want an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("Start() = %v", err)
			}

			for i, registrar := range registrars {
				if got := registrar.calls(); got != 1 {
					t.Errorf("service %d registered %d times, want 1", i, got)
				}
			}

			// The socket exists by the time Start returns, so a caller may connect
			// right away.
			conn, err := net.Dial("unix", address)
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

func TestValidateService(t *testing.T) {
	tests := []struct {
		name           string
		validator      validatorFunc // nil registers no service
		omitAddress    bool
		input          validatev1.Input
		stopBeforeCall bool
		wantStartErr   bool
		wantCode       codes.Code
		wantMessage    string
		wantErrors     string
		wantWarnings   validatev1.Violations
		wantCalled     bool
	}{
		{
			name:         "refuses to start without an address",
			validator:    validOutput,
			omitAddress:  true,
			wantStartErr: true,
		},
		{
			name:       "carries violations to the caller",
			validator:  violations,
			input:      bootstrapInput(),
			wantErrors: "Secret/d8-credentials: credential Secret is required",
			wantWarnings: validatev1.Violations{
				{Path: "NodeGroup/worker", Code: "replicas_zero", Message: "replicas is 0"},
			},
			wantCalled: true,
		},
		{
			name:       "reports a valid configuration as valid",
			validator:  validOutput,
			input:      bootstrapInput(),
			wantCalled: true,
		},
		{
			// The whitelist runs at the trust boundary, before the validator sees
			// the input.
			name:        "rejects an unknown operation",
			validator:   validOutput,
			input:       validatev1.Input{ProviderName: "dvp", Operation: "nonsense"},
			wantCode:    codes.InvalidArgument,
			wantMessage: `operation unknown: "nonsense"`,
		},
		{
			name:        "rejects a missing operation",
			validator:   validOutput,
			input:       validatev1.Input{ProviderName: "dvp"},
			wantCode:    codes.InvalidArgument,
			wantMessage: "operation required",
		},
		{
			// A panic is a bug, not a violation: without the stack the caller would
			// see a connection that died mid-call and could not say why.
			name:        "reports a panic as internal",
			validator:   panics,
			input:       bootstrapInput(),
			wantCode:    codes.Internal,
			wantMessage: "panic in",
			wantCalled:  true,
		},
		{
			name:        "reports a failed check as internal",
			validator:   fails,
			input:       bootstrapInput(),
			wantCode:    codes.Internal,
			wantMessage: errCheckFailed.Error(),
			wantCalled:  true,
		},
		{
			name:     "reports an unregistered action as unimplemented",
			input:    bootstrapInput(),
			wantCode: codes.Unimplemented,
		},
		{
			name:           "serves nothing after it is stopped",
			validator:      validOutput,
			input:          bootstrapInput(),
			stopBeforeCall: true,
			wantCode:       codes.Unavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false

			var services []server.Service
			if test.validator != nil {
				services = append(services, server.NewValidateService(validatorFunc(
					func(ctx context.Context, input validatev1.Input) (validatev1.Output, error) {
						called = true

						return test.validator(ctx, input)
					})))
			}

			address := ""
			if !test.omitAddress {
				address = socketPath(t)
			}

			running, err := server.Start(server.Config{Address: address}, services...)

			if test.wantStartErr {
				if err == nil {
					_ = running.Stop()
					t.Fatal("Start() = nil, want an error")
				}

				return
			}

			if err != nil {
				t.Fatalf("Start() = %v", err)
			}

			// Stop is idempotent, so this also covers the case that stops the
			// server itself.
			t.Cleanup(func() {
				if err := running.Stop(); err != nil {
					t.Errorf("Stop() = %v, want nil", err)
				}
			})

			validator := connect(t, address)

			if test.stopBeforeCall {
				if err := running.Stop(); err != nil {
					t.Fatalf("Stop() = %v", err)
				}

				requireUnavailable(t, validator, test.input)

				return
			}

			output, err := validator.Validate(context.Background(), test.input)

			if got := called; got != test.wantCalled {
				t.Errorf("validator called = %v, want %v", got, test.wantCalled)
			}

			if test.wantCode != codes.OK {
				requireStatus(t, err, test.wantCode, test.wantMessage)

				return
			}

			if err != nil {
				t.Fatalf("Validate() = %v", err)
			}

			if got := output.Errors.String(); got != test.wantErrors {
				t.Errorf("Errors().String() = %q, want %q", got, test.wantErrors)
			}

			if got := output.Warnings; !reflect.DeepEqual(got, test.wantWarnings) {
				t.Errorf("Warnings = %+v, want %+v", got, test.wantWarnings)
			}
		})
	}
}

// validatorFunc adapts a function to server.Validator, which a validator binary
// implements with a type of its own.
type validatorFunc func(ctx context.Context, input validatev1.Input) (validatev1.Output, error)

func (fn validatorFunc) Validate(ctx context.Context, input validatev1.Input) (validatev1.Output, error) {
	return fn(ctx, input)
}

var errCheckFailed = errors.New("dependency would not answer")

func validOutput(context.Context, validatev1.Input) (validatev1.Output, error) {
	return validatev1.Output{}, nil
}

func violations(context.Context, validatev1.Input) (validatev1.Output, error) {
	var output validatev1.Output
	output.AddError("Secret/d8-credentials", "credential_secret_required", "masked", "credential Secret is required")
	output.AddWarning("NodeGroup/worker", "replicas_zero", nil, "replicas is 0")

	return output, nil
}

func panics(context.Context, validatev1.Input) (validatev1.Output, error) {
	panic("validator bug")
}

func fails(context.Context, validatev1.Input) (validatev1.Output, error) {
	return validatev1.Output{}, errCheckFailed
}

func bootstrapInput() validatev1.Input {
	return validatev1.Input{ProviderName: "dvp", Operation: validatev1.OperationBootstrap}
}

// connect talks to the server the way dhctl would: over the wire, so the conversions
// and the statuses are exercised too.
func connect(t *testing.T, address string) client.Client {
	t.Helper()

	conn, err := grpc.NewClient("unix://"+address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return client.NewClient(conn, client.NewConfig())
}

func requireStatus(t *testing.T, err error, want codes.Code, message string) {
	t.Helper()

	if err == nil {
		t.Fatalf("Validate() = nil, want %s", want)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Validate() = %v, want a gRPC status", err)
	}

	if st.Code() != want {
		t.Fatalf("status = %s, want %s", st.Code(), want)
	}

	if message != "" && !strings.Contains(st.Message(), message) {
		t.Errorf("message = %q, want it to mention %q", st.Message(), message)
	}
}

// requireUnavailable waits for the socket to stop accepting after a Stop.
func requireUnavailable(t *testing.T, validator client.Client, input validatev1.Input) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		_, err := validator.Validate(ctx, input)
		if err != nil {
			requireStatus(t, err, codes.Unavailable, "")

			return
		}

		select {
		case <-ctx.Done():
			t.Fatal("server kept serving after Stop")
		case <-time.After(5 * time.Millisecond):
		}
	}
}
