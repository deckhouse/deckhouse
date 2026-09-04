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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
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
// real endpoint and the real gRPC call without building a fixture binary. What the
// spawned copy should be is carried in fakeConfigEnv, because the command line
// belongs to the protocol and a test must not add to it.
const fakeConfigEnv = "D8_TEST_VALIDATOR_CONFIG"

// fakeConfig is everything a test tells the validator it spawns: which of the fakes to
// be, and whatever that fake needs.
type fakeConfig struct {
	Mode Mode `json:"mode"`
	// ChildPidFile is where the fakes that spawn a helper write its pid, so a test
	// can ask whether the helper outlived the validator.
	ChildPidFile string `json:"childPidFile,omitempty"`
}

// Mode is which validator the test binary impersonates.
type Mode string

const (
	modeValid      Mode = "valid"
	modeViolations Mode = "violations"
	modeWarnings   Mode = "warnings" // reports something, but nothing that blocks
	modeBlank      Mode = "blank"    // rejects, but fills none of the violation fields
	modeLegacy     Mode = "legacy"   // a binary that knows no serve subcommand
	modeSlowStart  Mode = "slow"     // listens, but only after a while
	modeOrphan     Mode = "orphan"   // exits, leaving a child holding its output pipes
	modeChild      Mode = "child"    // serves, and spawns a child of its own
	modeStubborn   Mode = "stubborn" // listens, then ignores SIGTERM
)

// setFakeConfig points the validator the test spawns at one of the fakes above.
func setFakeConfig(t *testing.T, fake fakeConfig) {
	t.Helper()

	raw, err := json.Marshal(fake)
	if err != nil {
		t.Fatalf("Marshal(%+v) = %v", fake, err)
	}

	t.Setenv(fakeConfigEnv, string(raw))
}

// fakeConfigFromEnv reports what this copy of the test binary was spawned to be, and
// whether it was spawned as a validator at all.
func fakeConfigFromEnv() (fakeConfig, bool) {
	raw := os.Getenv(fakeConfigEnv)
	if raw == "" {
		return fakeConfig{}, false
	}

	var ret fakeConfig
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", fakeConfigEnv, err)
		os.Exit(1)
	}

	return ret, true
}

func TestMain(m *testing.M) {
	fake, spawned := fakeConfigFromEnv()
	if !spawned {
		os.Exit(m.Run())
	}

	// A bundle whose binary predates the gRPC protocol: it never gets as far as
	// reading the arguments.
	if fake.Mode == modeLegacy {
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", server.ServeCommand)

		os.Exit(1)
	}

	config, err := parseFakeValidatorArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}

	os.Exit(runFakeValidator(fake, config))
}

// parseFakeValidatorArgs reads the command line the way a real validator reads it, so
// a change to either side of the argv contract shows up here instead of in production.
func parseFakeValidatorArgs(args []string) (server.Config, error) {
	if len(args) == 0 {
		return server.Config{}, fmt.Errorf("no subcommand, want %s", server.ServeCommand)
	}

	if args[0] != server.ServeCommand {
		return server.Config{}, fmt.Errorf("unknown subcommand: %s", args[0])
	}

	flags := flag.NewFlagSet(server.ServeCommand, flag.ContinueOnError)
	configGetter := server.ConfigGetterFromFlags(flags)

	if err := flags.Parse(args[1:]); err != nil {
		return server.Config{}, err
	}

	return configGetter(), nil
}

func runFakeValidator(fake fakeConfig, config server.Config) int {
	// A validator that leaves a child behind: the child inherits stdout and stderr,
	// so the pipes stay open after the validator itself is gone.
	if fake.Mode == modeOrphan {
		child := exec.Command("sleep", "60")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		if err := child.Start(); err != nil {
			return 1
		}

		return 0
	}

	if fake.Mode == modeChild {
		child := exec.Command("sleep", "60")
		if err := child.Start(); err != nil {
			return 1
		}

		pid := []byte(strconv.Itoa(child.Process.Pid))
		if err := os.WriteFile(fake.ChildPidFile, pid, 0o600); err != nil {
			return 1
		}
	}

	if fake.Mode == modeSlowStart {
		time.Sleep(300 * time.Millisecond)
	}

	validator, err := server.Start(config, server.NewValidateService(newFakeValidator(fake.Mode)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// A validator that will not go down on SIGTERM: it listens and answers, but only
	// an actual kill ends it.
	if fake.Mode == modeStubborn {
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

// fakeValidator answers every call with the one response its mode stands for.
type fakeValidator struct {
	response *validatev1.ValidateResponse
}

func newFakeValidator(mode Mode) fakeValidator {
	switch mode {
	case modeBlank:
		return fakeValidator{response: &validatev1.ValidateResponse{
			Errors: []*validatev1.ViolationResponse{{}},
		}}

	case modeWarnings:
		return fakeValidator{response: &validatev1.ValidateResponse{
			Warnings: []*validatev1.ViolationResponse{{
				Path:    "DVPClusterConfiguration/layout",
				Code:    "layout_deprecated",
				Message: "layout is deprecated",
			}},
		}}

	case modeViolations:
		return fakeValidator{response: &validatev1.ValidateResponse{
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
		}}

	default:
		return fakeValidator{response: &validatev1.ValidateResponse{}}
	}
}

func (v fakeValidator) Validate(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	return v.response, nil
}

func convergeInput() config.ProviderInput {
	return config.ProviderInput{ProviderName: "dvp", Operation: string(validatev1.OperationConverge)}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
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
			// Warnings are for the operator to read, not a reason to stop: only an
			// error blocks the operation.
			name:  "a warning alone does not block the operation",
			mode:  modeWarnings,
			input: convergeInput(),
		},
		{
			// Fail closed: a rejection whose fields are all empty renders as no text,
			// and the blocking has to come from the violation being there at all —
			// not from the rendered text being non-empty.
			name:    "fails closed on a violation with no detail",
			mode:    modeBlank,
			input:   convergeInput(),
			wantErr: `provider "dvp" validation failed`,
		},
		{
			// Fail closed: a bundle whose binary predates the gRPC protocol blocks the
			// operation instead of counting as validated. Such a binary exits on the
			// unknown subcommand, and the caller learns it exited rather than waiting
			// out the readiness timeout.
			name:    "fails closed on a binary without the serve subcommand",
			mode:    modeLegacy,
			input:   convergeInput(),
			wantErr: "validator exited: exit status 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setFakeConfig(t, fakeConfig{Mode: test.mode})

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

// A validator that spawns helpers of its own must take them down with it: they run in
// its process group, and nothing reaps what the group leaves behind.
func TestStopTakesDownTheProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")

	setFakeConfig(t, fakeConfig{Mode: modeChild, ChildPidFile: pidFile})

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

	// The counter has to see a running validator, or its use below proves nothing.
	if running := countValidatorProcesses(t); running != 1 {
		t.Errorf("%d validators running while one is started, want 1", running)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}

	if leaked := countValidatorProcesses(t); leaked != 0 {
		t.Errorf("%d validators left running after Stop(), want 0", leaked)
	}

	child := readChildPid(t, pidFile)

	// Signal 0 only probes. The child is reparented when the validator dies, so it is
	// reaped by init rather than by us: give that a moment.
	for range 200 {
		if err := syscall.Kill(child, 0); err != nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("child %d of the validator still alive after Stop()", child)
}

func readChildPid(t *testing.T, path string) int {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v, want the pid the validator wrote", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("Atoi(%q) = %v", raw, err)
	}

	return pid
}

// countValidatorProcesses counts the validators still running: children of this test
// process whose name is the test binary. The mode a fake validator runs in is an
// environment variable, so matching on it would search command lines it never reaches.
// -x matches the process name exactly, which keeps pgrep from finding itself.
func countValidatorProcesses(t *testing.T) int {
	t.Helper()

	out, err := exec.Command(
		"pgrep",
		"-P", strconv.Itoa(os.Getpid()),
		"-x", filepath.Base(os.Args[0]),
	).Output()
	if err != nil {
		return 0
	}

	return len(strings.Fields(string(out)))
}

// Stop is called from withStop on a half-started process and again by the caller,
// so it has to survive both.
func TestValidatorProcessStopIsIdempotent(t *testing.T) {
	setFakeConfig(t, fakeConfig{Mode: modeValid})

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
	setFakeConfig(t, fakeConfig{Mode: modeStubborn})

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
	setFakeConfig(t, fakeConfig{Mode: modeValid})

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
	setFakeConfig(t, fakeConfig{Mode: modeOrphan})

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
