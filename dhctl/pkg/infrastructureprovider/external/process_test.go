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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/server"
)

// The test binary doubles as a provider validator: pointing binaryPath at os.Args[0]
// exercises the real spawn, the real endpoint and the real gRPC call without building
// a fixture binary. What the spawned copy should be travels in fakeConfigEnv, because
// the command line belongs to the protocol and a test must not add to it.
const fakeConfigEnv = "D8_TEST_VALIDATOR_CONFIG"

const (
	modeValid      mode = "valid"
	modeViolations mode = "violations"
	modeWarnings   mode = "warnings" // reports something, but nothing that blocks
	modeBlank      mode = "blank"    // rejects, but fills none of the violation fields
	modeLegacy     mode = "legacy"   // a binary that knows no serve subcommand
	modeSlowStart  mode = "slow"     // listens, but only after a while
	modeOrphan     mode = "orphan"   // exits, leaving a child holding its output pipes
	modeChild      mode = "child"    // serves, and spawns a child of its own
	modeSay        mode = "say"      // prints what it was told and exits
	modeStubborn   mode = "stubborn" // listens, then ignores SIGTERM
)

var errWaitFailed = errors.New("nothing worth waiting for")

// fakeConfig is everything a test tells the validator it spawns.
type fakeConfig struct {
	Mode mode `json:"mode"`
	// ChildPidFile is where a fake that spawns a helper writes its pid, so a test can
	// ask whether the helper outlived the validator.
	ChildPidFile string `json:"childPidFile,omitempty"`
	// SocketPath makes the fake serve on a unix socket instead of the endpoint it was
	// given, the way a provider may choose to.
	SocketPath string `json:"socketPath,omitempty"`
	// PidFile is where the fake writes its own pid, so a test can ask whether the
	// validator itself outlived a failed start.
	PidFile string `json:"pidFile,omitempty"`
	// SayOnStdout and SayOnStderr are printed before serving: a validator logs where
	// it pleases, and both streams have to reach the caller.
	SayOnStdout string `json:"sayOnStdout,omitempty"`
	SayOnStderr string `json:"sayOnStderr,omitempty"`
}

// mode is which validator the test binary impersonates.
type mode string

// fakeValidator answers every call with the one response its mode stands for.
type fakeValidator struct {
	response *validatev1.ValidateResponse
}

func TestMain(m *testing.M) {
	fake, spawned := fakeConfigFromEnv()
	if !spawned {
		os.Exit(m.Run())
	}

	// A binary that predates the protocol never gets as far as reading the arguments.
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

// A binary that cannot be started is an error, not a process handle.
func TestValidatorProcessStartRejectsAMissingBinary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := newValidatorProcess(ctx, validatorOptions{
		binaryPath: filepath.Join(t.TempDir(), "not-a-validator"),
		endpoint:   endpoint{Network: networkTCP, Address: loopbackAddress},
	})
	if err != nil {
		t.Fatalf("newValidatorProcess() = %v", err)
	}

	defer func() { _ = process.Stop() }()

	if _, err := process.Start(ctx); err == nil {
		t.Fatal("Start() = nil, want an error")
	}
}

// Options that could not start anything are refused before a process is spawned.
func TestNewValidatorProcessRefusesBadOptions(t *testing.T) {
	tests := []struct {
		name    string
		opt     validatorOptions
		wantErr string
	}{
		{
			name:    "without a binary",
			opt:     validatorOptions{},
			wantErr: "binary path is required",
		},
		{
			// Without the check the validator falls back to its own default and
			// listens somewhere the caller never asked for.
			name:    "without an address",
			opt:     validatorOptions{binaryPath: os.Args[0], endpoint: endpoint{Network: networkTCP}},
			wantErr: "endpoint: address is required",
		},
		{
			name:    "without a network",
			opt:     validatorOptions{binaryPath: os.Args[0], endpoint: endpoint{Address: loopbackAddress}},
			wantErr: "endpoint: network is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process, err := newValidatorProcess(context.Background(), test.opt)
			if process != nil {
				_ = process.Stop()
			}

			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("newValidatorProcess() = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

// A validator logs where it pleases, so both of its streams have to reach the caller.
// The fake prints and exits, and Stop is what guarantees everything it wrote was
// copied before the lines are read back.
func TestValidatorProcessReportsBothStreams(t *testing.T) {
	const (
		fromStdout = "a line on stdout"
		fromStderr = "a line on stderr"
	)

	setFakeConfig(t, fakeConfig{
		Mode:        modeSay,
		SayOnStdout: fromStdout,
		SayOnStderr: fromStderr,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu    sync.Mutex
		lines []string
	)

	collect := func(line string) {
		mu.Lock()
		defer mu.Unlock()

		lines = append(lines, line)
	}

	process, err := newValidatorProcess(ctx, validatorOptions{
		binaryPath:    os.Args[0],
		endpoint:      endpoint{Network: networkTCP, Address: loopbackAddress},
		stdoutHandler: collect,
		stderrHandler: collect,
	})
	if err != nil {
		t.Fatalf("newValidatorProcess() = %v", err)
	}

	processCtx, err := process.Start(ctx)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	// The context ends when the validator exits, which for this fake is right after
	// it has printed.
	<-processCtx.Done()

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !slices.Contains(lines, fromStdout) || !slices.Contains(lines, fromStderr) {
		t.Errorf("lines = %q, want both %q and %q", lines, fromStdout, fromStderr)
	}
}

// A start that gives up must not leave the validator running: the caller gets no handle
// to stop it with.
func TestListeningValidatorStartStopsTheValidatorThatNeverAnnounces(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "validator.pid")

	// The fake writes its pid and exits without ever announcing an endpoint.
	setFakeConfig(t, fakeConfig{Mode: modeSay, PidFile: pidFile})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validator, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	if _, err := validator.Start(ctx); err == nil {
		_ = validator.Stop()
		t.Fatal("Start() = nil, want the validator to announce nothing")
	}

	requireGone(t, readPid(t, pidFile))
}

// Bad options are refused before anything is spawned, and then there is no validator
// to hand back.
func TestNewListeningValidatorRefusesBadOptions(t *testing.T) {
	validator, err := newListeningValidator(context.Background(), "")
	if err == nil {
		_ = validator.Stop()
		t.Fatal("newListeningValidator() = nil, want an error")
	}

	if validator != nil {
		t.Errorf("newListeningValidator() = %v, want no validator", validator)
	}
}

// A validator logs before it binds, so the announcement is not the line the caller
// happens to read first.
func TestListeningValidatorCatchesAnEndpointAnnouncedLate(t *testing.T) {
	setFakeConfig(t, fakeConfig{Mode: modeValid, SayOnStdout: "starting up"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	validator, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	if _, err := validator.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
	}

	defer func() { _ = validator.Stop() }()

	if ep := validator.Endpoint(); ep.Network != networkTCP || ep.Address == "" {
		t.Errorf("endpoint = %s, want a tcp address", ep)
	}
}

// The endpoint a caller puts in the options is what the validator is told to listen on.
func TestValidatorCmdUsesTheGivenEndpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := validatorCmd(ctx, "/validator", endpoint{Network: networkUnix, Address: "/tmp/v.sock"}, io.Discard, io.Discard)

	want := []string{"/validator", server.ServeCommand, "--network=unix", "--address=/tmp/v.sock"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("Args = %q, want %q", cmd.Args, want)
	}
}

func TestValidatorOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opt     validatorOptions
		wantErr string
	}{
		{
			name: "a binary and an endpoint are what it takes",
			opt: validatorOptions{
				binaryPath: "/validator",
				endpoint:   endpoint{Network: networkTCP, Address: loopbackAddress},
			},
		},
		{
			name:    "without a binary there is nothing to start",
			opt:     validatorOptions{},
			wantErr: "binary path is required",
		},
		{
			name:    "an address without a network is half an endpoint",
			opt:     validatorOptions{binaryPath: "/validator", endpoint: endpoint{Address: loopbackAddress}},
			wantErr: "endpoint: network is required",
		},
		{
			name:    "and so is a network without an address",
			opt:     validatorOptions{binaryPath: "/validator", endpoint: endpoint{Network: networkTCP}},
			wantErr: "endpoint: address is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.opt.validate()

			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() = %v, want nil", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validate() = %v, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

// A validator that spawns helpers must take them down with it: nothing reaps what its
// process group leaves behind.
func TestStopTakesDownTheProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")

	setFakeConfig(t, fakeConfig{Mode: modeChild, ChildPidFile: pidFile})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	if _, err := process.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
	}

	validator := process.cmd.Process.Pid
	child := readPid(t, pidFile)

	// Signal 0 only probes. Both have to be alive here, or Stop proves nothing.
	if err := syscall.Kill(validator, 0); err != nil {
		t.Fatalf("Kill(%d, 0) = %v, want a running validator", validator, err)
	}

	if err := syscall.Kill(child, 0); err != nil {
		t.Fatalf("Kill(%d, 0) = %v, want a running child", child, err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}

	if err := syscall.Kill(validator, 0); err == nil {
		t.Errorf("validator %d still alive after Stop()", validator)
	}

	// The child is reparented when the validator dies, so init reaps it, not us.
	for range 200 {
		if err := syscall.Kill(child, 0); err != nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("child %d of the validator still alive after Stop()", child)
}

// SIGTERM is a request, not a guarantee: cmd.WaitDelay is what escalates to a kill.
func TestStopKillsAValidatorThatIgnoresSIGTERM(t *testing.T) {
	setFakeConfig(t, fakeConfig{Mode: modeStubborn})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	if _, err := process.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
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

	// cmd.Wait reaped it, so a live pid here would mean Stop left it behind.
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("validator pid %d still alive after Stop()", pid)
	}
}

// A validator that leaves a child holding its output pipes must not hang Stop: EOF
// never comes, so the wait has to be bounded.
func TestStopReturnsWhenOutputPipesStayOpen(t *testing.T) {
	setFakeConfig(t, fakeConfig{Mode: modeOrphan})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The validator exits at once, so the start fails; what matters is that it
	// returns at all instead of blocking on Stop.
	done := make(chan struct{})

	go func() {
		defer close(done)

		process, err := newListeningValidator(ctx, os.Args[0])
		if err != nil {
			return
		}

		if _, err := process.Start(ctx); err == nil {
			_ = process.Stop()
		}
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Stop() blocked on a child holding the output pipes")
	}
}

// Stop runs on a half-started process and again from the caller, so it has to survive
// both.
func TestValidatorProcessStopIsIdempotent(t *testing.T) {
	setFakeConfig(t, fakeConfig{Mode: modeValid})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	if _, err := process.Start(ctx); err != nil {
		t.Fatalf("Start() = %v", err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("second Stop() = %v, want nil", err)
	}
}

// The endpoint is whatever the validator announces, not what dhctl asked for: a
// provider may answer on a unix socket, and then the socket is what has to be dialled
// and cleaned up.
func TestValidatorOnAUnixSocket(t *testing.T) {
	// Not t.TempDir(): it names the directory after the test, and on darwin that
	// alone puts the socket over sun_path. dhctl's own tmp dir is short.
	tmpDir, err := os.MkdirTemp("", "d8t")
	if err != nil {
		t.Fatalf("MkdirTemp() = %v", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	socket := filepath.Join(tmpDir, "v.sock")

	setFakeConfig(t, fakeConfig{Mode: modeValid, SocketPath: socket})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := newListeningValidator(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("newListeningValidator() = %v", err)
	}

	ctx, err = process.Start(ctx)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}

	if ep := process.Endpoint(); ep.Network != networkUnix || ep.Address != socket {
		t.Fatalf("endpoint = %s, want unix://%s", ep, socket)
	}

	if _, err := requestValidate(ctx, process.Endpoint(), validatev1.Input{
		ProviderName: "dvp",
		Operation:    validatev1.OperationConverge,
	}); err != nil {
		t.Errorf("requestValidate() = %v", err)
	}

	if err := process.Stop(); err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}
}

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
	// The child inherits stdout and stderr, so they stay open after the validator is
	// gone.
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

	if fake.PidFile != "" {
		pid := []byte(strconv.Itoa(os.Getpid()))
		if err := os.WriteFile(fake.PidFile, pid, 0o600); err != nil {
			return 1
		}
	}

	if fake.SayOnStdout != "" {
		fmt.Fprintln(os.Stdout, fake.SayOnStdout)
	}

	if fake.SayOnStderr != "" {
		fmt.Fprintln(os.Stderr, fake.SayOnStderr)
	}

	if fake.Mode == modeSay {
		return 0
	}

	if fake.SocketPath != "" {
		config.Network = "unix"
		config.Address = fake.SocketPath
	}

	if fake.Mode == modeSlowStart {
		time.Sleep(300 * time.Millisecond)
	}

	validator, err := server.Start(config, server.NewValidateService(newFakeValidator(fake.Mode)))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// Listens and answers, but only an actual kill ends it.
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

func newFakeValidator(m mode) fakeValidator {
	switch m {
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

func waitForFile(t *testing.T, path string) {
	t.Helper()

	for range 200 {
		if _, err := os.Stat(path); err == nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("Stat(%s): the validator never wrote it", path)
}

func readPid(t *testing.T, path string) int {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("Atoi(%q) = %v", raw, err)
	}

	return pid
}

// requireGone probes with signal 0 until the process is reaped, which its parent may
// still be getting around to.
func requireGone(t *testing.T, pid int) {
	t.Helper()

	for range 200 {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("process %d still alive", pid)
}
