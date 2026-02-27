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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	runprogconf "github.com/criyle/go-sandbox/cmd/runprog/config"
	"github.com/criyle/go-sandbox/pkg/seccomp"
	sbseccomp "github.com/criyle/go-sandbox/pkg/seccomp/libseccomp"
	"github.com/criyle/go-sandbox/runner"
	"github.com/criyle/go-sandbox/runner/ptrace"
	"github.com/criyle/go-sandbox/runner/unshare"
)

const (
	sandboxCPUTimeLimit  = 2 * time.Second
	sandboxWallTimeLimit = 3 * time.Second
	sandboxMemoryLimit   = runner.Size(256 << 20) // 256 MiB
)

var sandboxExtraAllowSyscalls = []string{
	// Required by dynamically linked binaries and util-linux tools during startup.
	"set_tid_address",
	"set_robust_list",
	"futex",
	"rseq",
	"getpid",
	"gettid",
	"prlimit64",
	"getrandom",
	"getuid",
	"getgid",
	"geteuid",
	"getegid",
	"getppid",
	"uname",
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	if len(argv) > 0 && argv[0] == "--" {
		argv = argv[1:]
	}
	if len(argv) == 0 {
		log.Print("not enough arguments after --")
		return 1
	}

	nginxConfigPath := getNginxConfByArg("-c", argv)
	if nginxConfigPath == "" {
		log.Print("nginx config not found in args")
		return 1
	}

	realPathNginxConf, err := filepath.EvalSymlinks(nginxConfigPath)
	if err != nil {
		log.Printf("can't eval real config path for nginx config: %v", err)
		return 1
	}

	extraRead := []string{
		"/etc/nginx/",
		"/etc/passwd",
		"/etc/group",
		"/etc/nsswitch.conf",
		"/etc/hosts",
		"/etc/resolv.conf",
		"/usr/share/nginx/",
		"/chroot/",
		"/usr/bin/unshare",
		"/usr/local/nginx/sbin/nginx",
		"/chroot/usr/local/nginx/sbin/nginx",
		realPathNginxConf,
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed get pwd: %v", err)
		return 1
	}
	extraWrite := []string{
		"/dev/null",
		"/tmp/",
		"/dev/tty",
		"/chroot/etc/resolv.conf",
	}
	// Wrapper chain (`/usr/bin/nginx` shell script -> `unshare` -> nginx binary) needs fork/exec syscalls.
	args, allow, trace, handler := runprogconf.GetConf("default", workDir, argv, extraRead, extraWrite, true) // :contentReference[oaicite:4]{index=4}
	allow = appendUnique(allow, sandboxExtraAllowSyscalls...)

	limit := runner.Limit{
		TimeLimit:   sandboxCPUTimeLimit,
		MemoryLimit: sandboxMemoryLimit,
	}

	res, err := runWithPtrace(args, allow, trace, workDir, handler, limit, isDebug())
	if err != nil {
		log.Printf("seccomp build (ptrace): %v", err)
		return 1
	}

	if res.Status == runner.StatusRunnerError && strings.Contains(res.Error, "before execve") {
		log.Print("ptrace runner exited before execve; retrying with unshare runner")

		res, err = runWithUnshare(args, allow, trace, workDir, limit, isDebug())
		if err != nil {
			log.Printf("seccomp build (unshare): %v", err)
			return 1
		}
	}

	if res.Status == runner.StatusNormal && res.ExitStatus == 0 && res.Error == "" {
		return 0
	}

	log.Printf("sandbox run failed: status=%s exit_status=%d error=%q", res.Status, res.ExitStatus, res.Error)

	if res.Status == runner.StatusSignalled && res.ExitStatus > 0 && res.ExitStatus <= 127 {
		return 128 + res.ExitStatus
	}
	if res.ExitStatus != 0 {
		return res.ExitStatus
	}

	return 1
}

func runWithPtrace(
	args, allow, trace []string,
	workDir string,
	handler ptrace.Handler,
	limit runner.Limit,
	debug bool,
) (runner.Result, error) {
	filter, err := buildFilter(allow, trace, debug)
	if err != nil {
		return runner.Result{}, err
	}

	r := &ptrace.Runner{
		Args:    args,
		Env:     os.Environ(),
		WorkDir: workDir,
		Files:   []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Limit:   limit,
		Seccomp: filter,
		Handler: handler,
		// Debug
		ShowDetails: debug,
	}

	ctx, cancel := context.WithTimeout(context.Background(), sandboxWallTimeLimit)
	defer cancel()

	return r.Run(ctx), nil
}

func runWithUnshare(
	args, allow, trace []string,
	workDir string,
	limit runner.Limit,
	debug bool,
) (runner.Result, error) {
	// In unshare mode there is no ptrace handler, so traced syscalls must be explicitly allowed.
	allowAll := append(append([]string{}, allow...), trace...)
	filter, err := buildFilter(allowAll, nil, debug)
	if err != nil {
		return runner.Result{}, err
	}

	r := &unshare.Runner{
		Args:    args,
		Env:     os.Environ(),
		WorkDir: workDir,
		Files:   []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Limit:   limit,
		Seccomp: filter,
		// Debug
		ShowDetails: debug,
	}

	ctx, cancel := context.WithTimeout(context.Background(), sandboxWallTimeLimit)
	defer cancel()

	return r.Run(ctx), nil
}

func buildFilter(allow, trace []string, debug bool) (seccomp.Filter, error) {
	defaultAction := sbseccomp.ActionKill
	if debug {
		// In debug mode trace unknown syscalls to print their names instead of immediate SIGSYS.
		defaultAction = sbseccomp.ActionTrace
	}

	return (&sbseccomp.Builder{
		Allow:   allow,
		Trace:   trace,
		Default: defaultAction,
	}).Build()
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

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, v := range dst {
		seen[v] = struct{}{}
	}
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		dst = append(dst, v)
		seen[v] = struct{}{}
	}
	return dst
}
