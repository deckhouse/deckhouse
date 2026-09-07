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
	"io"
	"net"
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

// fakeConfig is everything a test tells the validator it spawns. It travels as JSON
// in fakeConfigEnv, so the spawned copy is configured the same way the test wrote it.
type fakeConfig struct {
	// Response is what the served validator answers with.
	Response *validatev1.ValidateResponse `json:"response,omitempty"`
	// SocketPath makes the fake serve on a unix socket instead of the endpoint it was
	// given, the way a provider may choose to.
	SocketPath string `json:"socketPath,omitempty"`
	// PidFile is where the fake writes its own pid, so a test can ask whether the
	// validator itself outlived a failed start.
	PidFile string `json:"pidFile,omitempty"`
	// ChildPidFile is where a fake that spawns a helper writes its pid, so a test can
	// ask whether the helper outlived the validator.
	ChildPidFile string `json:"childPidFile,omitempty"`
	// AnnounceAddress is announced as the endpoint instead of anything the fake bound,
	// so a test can point the caller at a socket the fake does not serve.
	AnnounceAddress string `json:"announceAddress,omitempty"`
	// SayOnStdout and SayOnStderr are printed before serving: a validator logs where
	// it pleases, and both streams have to reach the caller.
	SayOnStdout string `json:"sayOnStdout,omitempty"`
	SayOnStderr string `json:"sayOnStderr,omitempty"`
	// ListenAfter delays the bind, ExitAfter delays the exit of a fake that only
	// announces.
	ListenAfter time.Duration `json:"listenAfter,omitempty"`
	ExitAfter   time.Duration `json:"exitAfter,omitempty"`
	ExitCode    int           `json:"exitCode,omitempty"`
	// UnknownSubcommand impersonates a binary that predates the protocol.
	UnknownSubcommand bool `json:"unknownSubcommand,omitempty"`
	NoServe           bool `json:"noServe,omitempty"`
	HoldPipes         bool `json:"holdPipes,omitempty"`
	IgnoreSignals     bool `json:"ignoreSignals,omitempty"`
}

type fakeOption func(*fakeConfig)

func withConfig(fake fakeConfig) fakeOption {
	return func(c *fakeConfig) { *c = fake }
}

func withViolations() fakeOption {
	return func(c *fakeConfig) {
		c.Response = &validatev1.ValidateResponse{
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
		}
	}
}

func withWarnings() fakeOption {
	return func(c *fakeConfig) {
		c.Response = &validatev1.ValidateResponse{
			Warnings: []*validatev1.ViolationResponse{{
				Path:    "DVPClusterConfiguration/layout",
				Code:    "layout_deprecated",
				Message: "layout is deprecated",
			}},
		}
	}
}

// withBlankViolation rejects, but fills none of the violation fields.
func withBlankViolation() fakeOption {
	return func(c *fakeConfig) {
		c.Response = &validatev1.ValidateResponse{
			Errors: []*validatev1.ViolationResponse{{}},
		}
	}
}

func withUnknownSubcommand() fakeOption {
	return func(c *fakeConfig) { c.UnknownSubcommand = true }
}

func withListenAfter(delay time.Duration) fakeOption {
	return func(c *fakeConfig) { c.ListenAfter = delay }
}

func withoutServing() fakeOption {
	return func(c *fakeConfig) { c.NoServe = true }
}

func withPidFile(path string) fakeOption {
	return func(c *fakeConfig) { c.PidFile = path }
}

func withChildPidFile(path string) fakeOption {
	return func(c *fakeConfig) { c.ChildPidFile = path }
}

// withPipesHeldOpen leaves a child holding the output pipes after the fake is gone.
func withPipesHeldOpen() fakeOption {
	return func(c *fakeConfig) {
		c.HoldPipes = true
		c.NoServe = true
	}
}

func withIgnoredSignals() fakeOption {
	return func(c *fakeConfig) { c.IgnoreSignals = true }
}

func withSocket(path string) fakeOption {
	return func(c *fakeConfig) { c.SocketPath = path }
}

func withStdout(line string) fakeOption {
	return func(c *fakeConfig) { c.SayOnStdout = line }
}

func withStderr(line string) fakeOption {
	return func(c *fakeConfig) { c.SayOnStderr = line }
}

// withAnnouncedAddress announces an endpoint the fake never binds.
func withAnnouncedAddress(address string) fakeOption {
	return func(c *fakeConfig) { c.AnnounceAddress = address }
}

func withExitAfter(delay time.Duration, code int) fakeOption {
	return func(c *fakeConfig) {
		c.ExitAfter = delay
		c.ExitCode = code
	}
}

// fakeValidator answers every call with the one response it was given.
type fakeValidator struct {
	response *validatev1.ValidateResponse
}

func TestMain(m *testing.M) {
	fake, spawned := fakeConfigFromEnv()
	if !spawned {
		os.Exit(m.Run())
	}

	os.Exit(runFakeValidator(os.Args[1:], withConfig(fake)))
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

	setFakeConfig(t, withoutServing(), withStdout(fromStdout), withStderr(fromStderr))

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
	setFakeConfig(t, withoutServing(), withPidFile(pidFile))

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
	setFakeConfig(t, withStdout("starting up"))

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

	setFakeConfig(t, withChildPidFile(pidFile))

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
	setFakeConfig(t, withIgnoredSignals())

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
	setFakeConfig(t, withPipesHeldOpen())

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
	setFakeConfig(t)

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

	setFakeConfig(t, withSocket(socket))

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
// unservedAddress is a listening socket nobody accepts on: whoever connects waits for
// a server that never speaks.
func unservedAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen(networkTCP, loopbackAddress)
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	return listener.Addr().String()
}

func setFakeConfig(t *testing.T, opts ...fakeOption) {
	t.Helper()

	var fake fakeConfig
	for _, opt := range opts {
		opt(&fake)
	}

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

// runFakeValidator is the whole life of the spawned copy: it builds its config from
// the options it was given and does what that config asks for.
func runFakeValidator(args []string, opts ...fakeOption) int {
	var fake fakeConfig
	for _, opt := range opts {
		opt(&fake)
	}

	if fake.UnknownSubcommand {
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", server.ServeCommand)

		return 1
	}

	config, err := parseFakeValidatorArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	if fake.ChildPidFile != "" {
		child := exec.Command("sleep", "60")
		if err := child.Start(); err != nil {
			return 1
		}

		pid := []byte(strconv.Itoa(child.Process.Pid))
		if err := os.WriteFile(fake.ChildPidFile, pid, 0o600); err != nil {
			return 1
		}
	}

	// The child inherits stdout and stderr, so they stay open after the fake is gone.
	if fake.HoldPipes {
		child := exec.Command("sleep", "60")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		if err := child.Start(); err != nil {
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

	if fake.AnnounceAddress != "" {
		return announceAndDie(fake)
	}

	if fake.NoServe {
		return 0
	}

	return serveValidator(fake, config)
}

// announceAndDie is the validator that goes down: it never binds anything, points the
// caller at a socket of the test's own and exits while the call is still in flight.
func announceAndDie(fake fakeConfig) int {
	fmt.Println(server.InfoLine(server.InfoRecord{
		Network: networkTCP,
		Address: fake.AnnounceAddress,
	}))
	time.Sleep(fake.ExitAfter)
	fmt.Fprintln(os.Stderr, "validator failed after announcing")

	return fake.ExitCode
}

// serveValidator is the validator that stays up, built the way a provider builds one:
// the protocol library serves the answers until it is asked to stop.
func serveValidator(fake fakeConfig, config server.Config) int {
	if fake.SocketPath != "" {
		config.Network = networkUnix
		config.Address = fake.SocketPath
	}

	time.Sleep(fake.ListenAfter)

	validator, err := server.Start(config, server.NewValidateService(fakeValidator{response: fake.Response}))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// Listens and answers, but only an actual kill ends it.
	if fake.IgnoreSignals {
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

func (v fakeValidator) Validate(context.Context, validatev1.Input) (*validatev1.ValidateResponse, error) {
	if v.response == nil {
		return &validatev1.ValidateResponse{}, nil
	}

	return v.response, nil
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
