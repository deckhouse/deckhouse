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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	runprogconf "github.com/criyle/go-sandbox/cmd/runprog/config"
	sbseccomp "github.com/criyle/go-sandbox/pkg/seccomp/libseccomp"
	"github.com/criyle/go-sandbox/runner"
	"github.com/criyle/go-sandbox/runner/ptrace"
)

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
		"/usr/share/nginx/",
		realPathNginxConf,
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Printf("failed get pwd: %v", err)
		return 1
	}
	extraWrite := []string{"/dev/null", "/tmp/"}
	args, allow, trace, handler := runprogconf.GetConf("default", workDir, argv, extraRead, extraWrite, false) // :contentReference[oaicite:4]{index=4}

	filter, err := (&sbseccomp.Builder{
		Allow:   allow,
		Trace:   trace,
		Default: sbseccomp.ActionKill,
	}).Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seccomp build:", err)
	}

	// ExecFile need runner (fexecve)
	execF, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "open exec:", err)
		return 1
	}
	defer execF.Close()

	r := &ptrace.Runner{
		Args:        args,
		Env:         os.Environ(),
		WorkDir:     workDir,
		ExecFile:    execF.Fd(),
		Files:       []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Seccomp:     filter,
		Handler:     handler,
		ShowDetails: isDebug(), // Debug
	} // :contentReference[oaicite:5]{index=5}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res := r.Run(ctx)

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
