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

package external

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// The test binary doubles as a provider validator: Validate spawns whatever
// binaryPath points at, so pointing it at os.Args[0] exercises the real spawn, the
// real endpoint and the real gRPC call without building a fixture binary.
const modeEnv = "D8_TEST_VALIDATOR_MODE"

const (
	modeValid      = "valid"
	modeViolations = "violations"
	modeBlank      = "blank"    // rejects, but fills none of the violation fields
	modeLegacy     = "legacy"   // a binary that knows no serve subcommand
	modeSlowStart  = "slow"     // listens, but only after a while
	modeOrphan     = "orphan"   // exits, leaving a child holding its output pipes
	modeStubborn   = "stubborn" // listens, then ignores SIGTERM
)

func TestMain(m *testing.M) {
	mode := os.Getenv(modeEnv)
	if mode == "" {
		os.Exit(m.Run())
	}
	os.Exit(runFakeValidator(mode))
}

func runFakeValidator(mode string) int {
	if mode == modeLegacy {
		fmt.Fprintln(os.Stderr, "unknown subcommand: serve")

		return 1
	}

	network, address := "", ""

	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, networkFlag):
			network = strings.TrimPrefix(arg, networkFlag)
		case strings.HasPrefix(arg, addressFlag):
			address = strings.TrimPrefix(arg, addressFlag)
		}
	}

	// A validator that leaves a child behind: the child inherits stdout and stderr,
	// so the pipes stay open after the validator itself is gone.
	if mode == modeOrphan {
		child := exec.Command("sleep", "60")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		if err := child.Start(); err != nil {
			return 1
		}

		return 0
	}

	if mode == modeSlowStart {
		time.Sleep(300 * time.Millisecond)
	}

	validator, err := server.Start(
		server.Config{Network: network, Address: address},
		server.NewValidateService(fakeValidator(mode)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// A validator that will not go down on SIGTERM: it listens and answers, but only
	// an actual kill ends it.
	if mode == modeStubborn {
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
		time.Sleep(5 * time.Minute)

		return 0
	}

	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	<-stopped

	if err := validator.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return 0
}

type fakeValidator string

func (mode fakeValidator) Validate(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	if mode == modeBlank {
		return &validatev1.ValidateResponse{
			Errors: []*validatev1.ViolationResponse{{}},
		}, nil
	}

	if mode == modeViolations {
		return &validatev1.ValidateResponse{
			Errors: []*validatev1.ViolationResponse{{
				Path:    "Secret/d8-credentials",
				Code:    "credential_secret_required",
				Message: "credential Secret is required",
			}},
			Warnings: []*validatev1.ViolationResponse{{
				Path:    "NodeGroup/worker",
				Code:    "replicas_zero",
				Message: "replicas is 0",
			}},
		}, nil
	}

	return &validatev1.ValidateResponse{}, nil
}

func convergeInput() config.ProviderInput {
	return config.ProviderInput{ProviderName: "dvp", Operation: string(validatev1.OperationConverge)}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		input   config.ProviderInput
		wantErr string
	}{
		{
			name:  "passes a valid configuration",
			mode:  modeValid,
			input: convergeInput(),
		},
		{
			// The validator needs a moment to bind: the caller must wait for it
			// rather than dial into a port nobody listens on yet.
			name:  "waits for a validator that takes a while to listen",
			mode:  modeSlowStart,
			input: convergeInput(),
		},
		{
			name:    "reports violations as the error text",
			mode:    modeViolations,
			input:   convergeInput(),
			wantErr: "Secret/d8-credentials: credential Secret is required",
		},
		{
			// Fail closed: a rejection whose fields are all empty renders as no text,
			// and the blocking has to come from the violation being there at all —
			// not from the rendered text being non-empty.
			name:    "fails closed on a violation with no detail",
			mode:    modeBlank,
			input:   convergeInput(),
			wantErr: "provider validation failed",
		},
		{
			// Fail closed: a bundle whose binary predates the gRPC protocol blocks
			// the operation instead of counting as validated.
			name:    "fails closed on a binary without the serve subcommand",
			mode:    modeLegacy,
			input:   convergeInput(),
			wantErr: "did not start",
		},
		{
			name:    "fails closed on an unknown operation",
			mode:    modeValid,
			input:   config.ProviderInput{ProviderName: "dvp", Operation: "nonsense"},
			wantErr: "operation unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(modeEnv, test.mode)

			err := Validate(context.Background(), os.Args[0], test.input)

			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", test.wantErr)
			}

			if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("Validate() = %q, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

// A validation must not leave the validator running: dhctl calls it several times per
// run, and every spawn that outlives its call is a process nobody reaps.
func TestValidateStopsTheProcess(t *testing.T) {
	t.Setenv(modeEnv, modeValid)

	ep, err := NewTCPEndpoint()
	if err != nil {
		t.Fatalf("NewTCPEndpoint() = %v", err)
	}

	defer func() { _ = ep.Free() }()

	if err := Validate(context.Background(), os.Args[0], convergeInput()); err != nil {
		t.Fatalf("Validate() = %v", err)
	}

	// Whatever endpoint the validation used, nothing must answer on it afterwards;
	// a leaked process would still hold its port.
	if leaked := countValidatorProcesses(t); leaked != 0 {
		t.Errorf("%d validator processes left running, want 0", leaked)
	}
}

func countValidatorProcesses(t *testing.T) int {
	t.Helper()

	out, err := exec.Command("pgrep", "-f", modeEnv).Output()
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(out)))
}

// Stop is called from withStop on a half-started process and again by the caller,
// so it has to survive both.
func TestValidatorProcessStopIsIdempotent(t *testing.T) {
	t.Setenv(modeEnv, modeValid)

	ep, err := NewTCPEndpoint()
	if err != nil {
		t.Fatalf("NewTCPEndpoint() = %v", err)
	}

	defer func() { _ = ep.Free() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := StartValidatorProcess(ctx, os.Args[0], ep)
	if err != nil {
		t.Fatalf("StartValidatorProcess() = %v", err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("second Stop() = %v, want nil", err)
	}
}

// SIGTERM is a request, not a guarantee. A validator that ignores it must still be
// gone when Stop returns: cmd.WaitDelay is what escalates to a kill, and it only
// applies because Stop goes through cmd.Wait.
func TestStopKillsAValidatorThatIgnoresSIGTERM(t *testing.T) {
	t.Setenv(modeEnv, modeStubborn)

	ep, err := NewTCPEndpoint()
	if err != nil {
		t.Fatalf("NewTCPEndpoint() = %v", err)
	}

	defer func() { _ = ep.Free() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := StartValidatorProcess(ctx, os.Args[0], ep)
	if err != nil {
		t.Fatalf("StartValidatorProcess() = %v", err)
	}

	pid := process.cmd.Process.Pid

	done := make(chan error, 1)

	go func() { done <- process.Stop() }()

	select {
	case err := <-done:
		// Having to kill it is not a failure the caller can do anything about.
		if err != nil {
			t.Errorf("Stop() = %v, want nil", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Stop() blocked on a validator that ignores SIGTERM")
	}

	// Signal 0 only probes: the process is reaped by cmd.Wait, so a live pid here
	// would mean Stop left it behind.
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("validator pid %d still alive after Stop()", pid)
	}
}

// The protocol accepts a unix socket too; dhctl uses TCP, but the endpoint must work
// so a caller can switch to it.
func TestUnixEndpointServesValidator(t *testing.T) {
	t.Setenv(modeEnv, modeValid)

	// Not t.TempDir(): it names the directory after the test, and on darwin that
	// alone puts the socket over sun_path. dhctl's own tmp dir is short.
	tmpDir, err := os.MkdirTemp("", "d8t")
	if err != nil {
		t.Fatalf("MkdirTemp() = %v", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	ep, err := NewUnixEndpoint(tmpDir)
	if err != nil {
		t.Fatalf("NewUnixEndpoint() = %v", err)
	}

	defer func() { _ = ep.Free() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := StartValidatorProcess(ctx, os.Args[0], ep)
	if err != nil {
		t.Fatalf("StartValidatorProcess() = %v", err)
	}

	defer func() { _ = process.Stop() }()

	// StartValidatorProcess returns a validator that already listens.
	if _, err := requestValidate(ctx, ep, validatev1.Input{
		ProviderName: "dvp",
		Operation:    validatev1.OperationConverge,
	}); err != nil {
		t.Errorf("requestValidate() = %v", err)
	}
}

// A validator that leaves a child holding its output pipes must not hang Stop: EOF
// never comes, so the wait has to be bounded.
func TestStopReturnsWhenOutputPipesStayOpen(t *testing.T) {
	t.Setenv(modeEnv, modeOrphan)

	ep, err := NewTCPEndpoint()
	if err != nil {
		t.Fatalf("NewTCPEndpoint() = %v", err)
	}

	defer func() { _ = ep.Free() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The validator exits at once, so the start fails; what matters is that it
	// returns at all instead of blocking on Stop.
	done := make(chan struct{})

	go func() {
		defer close(done)

		process, err := StartValidatorProcess(ctx, os.Args[0], ep)
		if err == nil {
			_ = process.Stop()
		}
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Stop() blocked on a child holding the output pipes")
	}
}
