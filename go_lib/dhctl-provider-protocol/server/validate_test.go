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
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
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

var errCheckFailed = errors.New("dependency would not answer")

const panicValue = "validator bug: kubeconfigDataBase64 c2VjcmV0"

func TestValidateService(t *testing.T) {
	tests := []struct {
		name           string
		validator      validatorFunc // nil registers no service
		input          validatev1.Input
		stopBeforeCall bool
		wantCode       codes.Code
		wantMessage    string
		wantErrors     string
		wantWarnings   []string
		wantCalled     bool
	}{
		{
			name:         "carries violations to the caller",
			validator:    violations,
			input:        bootstrapInput(),
			wantErrors:   "Secret/d8-credentials: credential Secret is required",
			wantWarnings: []string{"NodeGroup/worker: replicas is 0"},
			wantCalled:   true,
		},
		{
			name:       "reports a valid configuration as valid",
			validator:  validConfiguration,
			input:      bootstrapInput(),
			wantCalled: true,
		},
		{
			// A panic is a bug, not a violation: the caller gets the method, while
			// the value and its stack stay in the log — a panic may quote the
			// request, and the request carries credentials.
			name:        "reports a panic as internal",
			validator:   panics,
			input:       bootstrapInput(),
			wantCode:    codes.Internal,
			wantMessage: "panic in",
			wantCalled:  true,
		},
		{
			// A plain error means the check could not be made, which is the server's
			// fault as far as the caller can tell. Its text has to survive: it is all
			// the caller gets to explain why nothing was validated.
			name:        "reports a failed check as internal",
			validator:   fails,
			input:       bootstrapInput(),
			wantCode:    codes.Internal,
			wantMessage: errCheckFailed.Error(),
			wantCalled:  true,
		},
		{
			name:        "reports an empty answer as internal",
			validator:   returnsNothing,
			input:       bootstrapInput(),
			wantCode:    codes.Internal,
			wantMessage: "validator returned no response",
			wantCalled:  true,
		},
		{
			name:     "reports an unregistered action as unimplemented",
			input:    bootstrapInput(),
			wantCode: codes.Unimplemented,
		},
		{
			name:           "serves nothing after it is stopped",
			validator:      validConfiguration,
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
					func(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
						called = true

						return test.validator(ctx, input)
					})))
			}

			// No address: the default puts the server on loopback and Addr says
			// which port it got.
			running, err := server.Start(server.Config{}, services...)
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

			validator := connect(t, running.Addr().String())

			if test.stopBeforeCall {
				if err := running.Stop(); err != nil {
					t.Fatalf("Stop() = %v", err)
				}

				requireUnavailable(t, validator, test.input)

				return
			}

			resp, err := validator.Validate(context.Background(), test.input)

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

			if got := violationsText(resp.GetErrors()); got != test.wantErrors {
				t.Errorf("errors text = %q, want %q", got, test.wantErrors)
			}

			var warnings []string
			for _, warning := range resp.GetWarnings() {
				warnings = append(warnings, violationsText([]*validatev1.ViolationResponse{warning}))
			}

			if !reflect.DeepEqual(warnings, test.wantWarnings) {
				t.Errorf("warnings = %q, want %q", warnings, test.wantWarnings)
			}
		})
	}
}

// A panic value may quote the request, and the request carries credentials, so only
// the method reaches the caller.
func TestValidatePanicStatusCarriesNoPanicValue(t *testing.T) {
	running, err := server.Start(
		server.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		server.NewValidateService(validatorFunc(panics)),
	)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	t.Cleanup(func() {
		if err := running.Stop(); err != nil {
			t.Errorf("Stop() = %v, want nil", err)
		}
	})

	_, err = connect(t, running.Addr().String()).Validate(context.Background(), bootstrapInput())
	if err == nil {
		t.Fatal("Validate() = nil, want a panic reported as internal")
	}

	if strings.Contains(err.Error(), panicValue) {
		t.Errorf("Validate() = %q, want a message without the panic value", err)
	}
}

func TestValidatePanicStatusCarriesNoStack(t *testing.T) {
	var logged bytes.Buffer

	running, err := server.Start(
		server.Config{Logger: slog.New(slog.NewTextHandler(&logged, nil))},
		server.NewValidateService(validatorFunc(panics)),
	)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	t.Cleanup(func() {
		if err := running.Stop(); err != nil {
			t.Errorf("Stop() = %v, want nil", err)
		}
	})

	_, err = connect(t, running.Addr().String()).Validate(context.Background(), bootstrapInput())
	if err == nil {
		t.Fatal("Validate() = nil, want a panic reported as internal")
	}

	if strings.Contains(err.Error(), "goroutine ") {
		t.Errorf("Validate() = %q, want a message without the stack", err)
	}

	if got := logged.String(); !strings.Contains(got, "goroutine ") {
		t.Errorf("configured logger got %q, want the panic stack", got)
	}
}

// validatorFunc adapts a function to server.Validator, which a validator binary
// implements with a type of its own.
type validatorFunc func(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error)

func (fn validatorFunc) Validate(ctx context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	return fn(ctx, input)
}

func validConfiguration(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return &validatev1.ValidateResponse{}, nil
}

func violations(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return &validatev1.ValidateResponse{
		Errors: []*validatev1.ViolationResponse{{
			Path:    "Secret/d8-credentials",
			Code:    "credential_secret_required",
			Message: "credential Secret is required",
			Value:   "masked",
		}},
		Warnings: []*validatev1.ViolationResponse{{
			Path:    "NodeGroup/worker",
			Code:    "replicas_zero",
			Message: "replicas is 0",
		}},
	}, nil
}

func returnsNothing(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return nil, nil
}

func panics(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	panic(panicValue)
}

func fails(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return nil, errCheckFailed
}

func bootstrapInput() validatev1.Input {
	return validatev1.Input{ProviderName: "dvp", Operation: validatev1.OperationBootstrap}
}

// connect talks to the server the way dhctl would: over the wire on loopback, so the
// conversions and the statuses are exercised too.
func connect(t *testing.T, address string) client.ValidateClient {
	t.Helper()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return client.NewValidateClient(conn, client.NewConfig())
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
func requireUnavailable(t *testing.T, validator client.ValidateClient, input validatev1.Input) {
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

// violationsText renders violations the way a caller would print them.
func violationsText(violations []*validatev1.ViolationResponse) string {
	lines := make([]string, 0, len(violations))

	for _, violation := range violations {
		if violation.GetPath() == "" {
			lines = append(lines, violation.GetMessage())

			continue
		}

		lines = append(lines, violation.GetPath()+": "+violation.GetMessage())
	}

	return strings.Join(lines, "\n")
}
