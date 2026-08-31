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
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/client"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/validate"
)

// exampleValidator is what a plugin author writes. A Result carries what is wrong
// with the configuration; an error means the plugin could not decide and reaches
// the host as Internal.
type exampleValidator struct{}

func (exampleValidator) Validate(_ context.Context, input validate.Input) (validate.Output, error) {
	var result validate.Output

	if input.CloudProviderVars == nil || len(input.CloudProviderVars.Secrets) == 0 {
		result.AddError("Secret/d8-credentials", "credential_secret_required", nil,
			`credential Secret "d8-credentials" is required`)
	}

	return result, nil
}

// Example is a whole plugin binary: the host invokes it as
// `validator serve --address=<unix socket>` and stops it with SIGTERM.
func Example() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintf(os.Stderr, "usage: %s serve --address=<unix socket>\n", os.Args[0])
		os.Exit(2)
	}

	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	address := flags.String("address", "", "unix socket to serve on")
	_ = flags.Parse(os.Args[2:])

	// Serve installs no signal handler: the binary decides what SIGTERM means.
	ctx, unhook := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer unhook()

	service := server.NewValidateService(exampleValidator{})

	stop, err := server.Start(ctx, server.Config{Address: *address}, service)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	<-ctx.Done()

	if err := stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// startPlugin talks to the handlers the way a host would: over the wire, so the
// conversions and the statuses are exercised too. The returned cancel stops the
// server through its context instead of through stop.
func startPlugin(t *testing.T, services ...server.Service) (client.Client, context.CancelFunc) {
	t.Helper()

	// Not t.TempDir: it spells the test's name into the path, which alone can
	// exceed sun_path (104 bytes on darwin, 108 on Linux) and fail bind.
	dir, err := os.MkdirTemp("", "d8v")
	if err != nil {
		t.Fatalf("MkdirTemp() = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ctx, cancel := context.WithCancel(context.Background())

	stop, err := server.Start(ctx, server.Config{Address: filepath.Join(dir, "v.sock")}, services...)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Errorf("stop() = %v, want nil", err)
		}
	})

	conn, err := grpc.NewClient("unix://"+filepath.Join(dir, "v.sock"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return client.NewClient(conn), cancel
}

// validatorFunc adapts a function to validate.Validator, which a plugin implements
// with a type of its own.
type validatorFunc func(ctx context.Context, input validate.Input) (validate.Output, error)

func (fn validatorFunc) Validate(ctx context.Context, input validate.Input) (validate.Output, error) {
	return fn(ctx, input)
}

func validating(fn validatorFunc) server.Service {
	return server.NewValidateService(fn)
}

func validInput() validate.Input {
	return validate.Input{ProviderName: "dvp", Operation: validate.OperationBootstrap}
}

func TestServeCarriesViolations(t *testing.T) {
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		var result validate.Output
		result.AddError("Secret/d8-credentials", "credential_secret_required", "masked", "credential Secret is required")
		result.AddWarning("NodeGroup/worker", "replicas_zero", nil, "replicas is 0")
		return result, nil
	}))

	result, err := plugin.Validate(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Validate() = %v", err)
	}

	if got := result.Errors().String(); got != "Secret/d8-credentials: credential Secret is required" {
		t.Errorf("Errors().String() = %q", got)
	}
	if got := result.Errors()[0].Value; got != "masked" {
		t.Errorf("Value = %v, want %q", got, "masked")
	}
	if got := len(result.Warnings()); got != 1 {
		t.Errorf("Warnings() = %d, want 1", got)
	}
}

func TestServeEmptyResultIsValid(t *testing.T) {
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		return validate.Output{}, nil
	}))

	result, err := plugin.Validate(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if result.HasErrors() {
		t.Errorf("HasErrors() = true, want a valid result: %s", result.Errors().String())
	}
}

func TestServeUnknownOperationRejectedBeforePlugin(t *testing.T) {
	called := false
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		called = true
		return validate.Output{}, nil
	}))

	_, err := plugin.Validate(context.Background(), validate.Input{ProviderName: "dvp", Operation: "nonsense"})
	if got := statusCode(t, err); got != codes.InvalidArgument {
		t.Errorf("status = %s, want %s", got, codes.InvalidArgument)
	}
	if called {
		t.Error("plugin was called with an unknown operation")
	}
	if got := statusMessage(t, err); got != `invalid request: operation unknown: "nonsense"` {
		t.Errorf("message = %q", got)
	}
}

func TestServeMissingOperationRejected(t *testing.T) {
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		return validate.Output{}, nil
	}))

	_, err := plugin.Validate(context.Background(), validate.Input{ProviderName: "dvp"})
	if got := statusCode(t, err); got != codes.InvalidArgument {
		t.Errorf("status = %s, want %s", got, codes.InvalidArgument)
	}
}

func TestServePanicBecomesInternal(t *testing.T) {
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		panic("plugin bug")
	}))

	_, err := plugin.Validate(context.Background(), validInput())
	if got := statusCode(t, err); got != codes.Internal {
		t.Fatalf("status = %s, want %s", got, codes.Internal)
	}
	if got := statusMessage(t, err); !strings.Contains(got, "plugin bug") || !strings.Contains(got, "serve_test.go") {
		t.Errorf("message = %q, want the panic value and a stack", got)
	}
}

func TestServePluginErrorBecomesInternal(t *testing.T) {
	plugin, _ := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		return validate.Output{}, errPluginFailed
	}))

	_, err := plugin.Validate(context.Background(), validInput())
	if got := statusCode(t, err); got != codes.Internal {
		t.Errorf("status = %s, want %s", got, codes.Internal)
	}
}

func TestServeWithoutValidatorIsUnimplemented(t *testing.T) {
	plugin, _ := startPlugin(t)

	_, err := plugin.Validate(context.Background(), validInput())
	if got := statusCode(t, err); got != codes.Unimplemented {
		t.Errorf("status = %s, want %s", got, codes.Unimplemented)
	}
}

// stop() returning nil after the context stopped the server is checked by
// startPlugin's cleanup.
func TestServeStopsOnContextCancel(t *testing.T) {
	plugin, cancel := startPlugin(t, validating(func(_ context.Context, _ validate.Input) (validate.Output, error) {
		return validate.Output{}, nil
	}))

	if _, err := plugin.Validate(context.Background(), validInput()); err != nil {
		t.Fatalf("Validate() before shutdown = %v", err)
	}

	cancel()

	ctx, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWait()

	for {
		if _, err := plugin.Validate(ctx, validInput()); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("server kept serving after the context was cancelled")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func statusCode(t *testing.T, err error) codes.Code {
	t.Helper()
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", err)
	}
	return st.Code()
}

func statusMessage(t *testing.T, err error) string {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", err)
	}
	return st.Message()
}

var errPluginFailed = errors.New("dependency would not answer")
