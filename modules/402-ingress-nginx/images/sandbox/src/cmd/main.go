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

package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	runprogconf "github.com/criyle/go-sandbox/cmd/runprog/config"
	"github.com/criyle/go-sandbox/pkg/forkexec"
	"github.com/criyle/go-sandbox/pkg/seccomp"
	sbseccomp "github.com/criyle/go-sandbox/pkg/seccomp/libseccomp"
	"github.com/criyle/go-sandbox/ptracer"
	"github.com/criyle/go-sandbox/runner"
	sbptrace "github.com/criyle/go-sandbox/runner/ptrace"
	"golang.org/x/sys/unix"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	mode, argv := parseSandboxMode(argv)
	debug := isDebug()
	debugCrashOnDeny := isDebugCrashOnDeny()
	argv = normalizeSandboxArgs(argv)
	if len(argv) == 0 {
		log.Print("not enough arguments after --")
		return 1
	}

	nginxConfigPath := getNginxConfByArg("-c", argv)
	if nginxConfigPath == "" {
		log.Print("nginx config not found in args")
		return 1
	}

	extraRead := getSandboxExtraRead(nginxConfigPath)
	extraWrite := getSandboxExtraWrite()

	workDir := ""
	switch mode {
	case sandboxModeIsolatedProcess:
		return runIsolatedProcessHelper(argv)
	case sandboxModeIsolatedProcessChild:
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := setupIsolatedProcessChild(); err != nil {
			log.Printf("failed to setup isolated process mode: %v", err)
			return 1
		}
		workDir = "/"
	default:
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			log.Printf("failed get pwd: %v", err)
			return 1
		}
	}

	args, allow, trace, handler := runprogconf.GetConf("default", workDir, argv, extraRead, extraWrite, true) // :contentReference[oaicite:4]{index=4}
	var traceHandler sbptrace.Handler = handler
	if debug {
		traceHandler = withDebugHandler(handler, debugCrashOnDeny)
	}
	allow = append(allow, sandboxExtraAllowSyscalls...)

	limit := runner.Limit{
		TimeLimit:   sandboxCPUTimeLimit,
		MemoryLimit: sandboxMemoryLimit,
	}

	res, err := runWithPtrace(args, allow, trace, workDir, traceHandler, limit, debug)
	if err != nil {
		log.Printf("seccomp build (ptrace): %v", err)
		return 1
	}

	if debug {
		log.Printf(
			"sandbox metrics: status=%s exit=%d cpu=%s wall=%s mem=%dB err=%q",
			res.Status, res.ExitStatus, res.Time, res.RunningTime, res.Memory, res.Error,
		)
	}

	if res.Status == runner.StatusNormal && res.ExitStatus == 0 && res.Error == "" {
		return 0
	}

	log.Printf("sandbox run failed: status=%s exit_status=%d error=%q", res.Status, res.ExitStatus, res.Error)

	if res.Status == runner.StatusSignalled && res.ExitStatus > 0 && res.ExitStatus <= 127 {
		exitCode := 128 + res.ExitStatus
		log.Printf("sandbox exit mapping: status=Signalled signal=%d mapped_exit_code=%d", res.ExitStatus, exitCode)
		return exitCode
	}
	if res.ExitStatus != 0 {
		log.Printf("sandbox exit mapping: propagating non-zero exit_status=%d", res.ExitStatus)
		return res.ExitStatus
	}

	log.Print("sandbox exit mapping: fallback to exit_code=1")
	return 1
}

func runWithPtrace(
	args, allow, trace []string,
	workDir string,
	handler sbptrace.Handler,
	limit runner.Limit,
	debug bool,
) (runner.Result, error) {
	filter, err := buildFilter(allow, trace)
	if err != nil {
		return runner.Result{}, err
	}

	r := &forkexec.Runner{
		Args:     args,
		Env:      os.Environ(),
		WorkDir:  workDir,
		Files:    []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Seccomp:  filter.SockFprog(),
		Ptrace:   true,
		SyncFunc: nil,

		UnshareCgroupAfterSync: os.Getuid() == 0,
	}
	traceHandler := newSandboxTraceHandler(handler, debug)
	tracer := ptracer.Tracer{
		Handler: traceHandler,
		Runner:  r,
		Limit:   limit,
	}

	ctx, cancel := context.WithTimeout(context.Background(), sandboxWallTimeLimit)
	defer cancel()

	return tracer.Trace(ctx), nil
}

func buildFilter(allow, trace []string) (seccomp.Filter, error) {
	return (&sbseccomp.Builder{
		Allow: allow,
		Trace: trace,
		// Trace unknown syscalls so production mode can log the denied syscall name
		// before converting it into a hard failure in the ptrace handler.
		Default: sbseccomp.ActionTrace,
	}).Build()
}

func normalizeSandboxArgs(argv []string) []string {
	if len(argv) > 0 && argv[0] == "--" {
		return argv[1:]
	}
	return argv
}

type sandboxMode uint8

const (
	sandboxModeDefault sandboxMode = iota
	sandboxModeIsolatedProcess
	sandboxModeIsolatedProcessChild
)

func parseSandboxMode(argv []string) (sandboxMode, []string) {
	if len(argv) > 0 && argv[0] == "--isolated-process" {
		return sandboxModeIsolatedProcess, argv[1:]
	}
	if len(argv) > 0 && argv[0] == "--isolated-process-child" {
		return sandboxModeIsolatedProcessChild, argv[1:]
	}

	return sandboxModeDefault, argv
}

func runIsolatedProcessHelper(argv []string) int {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("failed to resolve sandbox executable: %v", err)
		return 1
	}

	// The helper mode creates an exec boundary before entering user/net namespaces.
	// Unsharing CLONE_NEWUSER from the already running Go sandbox process is unreliable
	// because the runtime may already have multiple OS threads.
	childArgs := append([]string{"--isolated-process-child", "--"}, argv...)
	cmd := exec.Command(exe, childArgs...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{{
			ContainerID: os.Getuid(),
			HostID:      os.Getuid(),
			Size:        1,
		}},
		GidMappings: []syscall.SysProcIDMap{{
			ContainerID: os.Getgid(),
			HostID:      os.Getgid(),
			Size:        1,
		}},
		GidMappingsEnableSetgroups: false,
		// The child must be able to bring loopback up, chroot into /validation-chroot
		// and keep low-port bind capability for the final nginx exec in the private netns.
		AmbientCaps: []uintptr{
			unix.CAP_NET_ADMIN,
			unix.CAP_NET_BIND_SERVICE,
			unix.CAP_SYS_CHROOT,
		},
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		log.Printf("failed to run isolated process child: %v", err)
		return 1
	}

	return 0
}

// getNginxConfByArg return parametr args of nginx, as sample for `-c` flag return path config
func getNginxConfByArg(arg string, args []string) string {
	for i := 1; i < len(args); i++ {
		if args[i] == arg && i+1 < len(args) {
			return args[i+1]
		}
	}

	return ""
}

func isDebug() bool {
	val := os.Getenv("SANDBOX_DEBUG")
	debug, err := strconv.ParseBool(val)
	if err != nil {
		return false // Default value if unset or invalid
	}
	return debug
}

func isDebugCrashOnDeny() bool {
	val := os.Getenv("SANDBOX_DEBUG_CRASH_ON_DENY")
	crashOnDeny, err := strconv.ParseBool(val)
	if err != nil {
		return false // Default value if unset or invalid
	}
	return crashOnDeny
}
