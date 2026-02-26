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
	"github.com/criyle/go-sandbox/runner/ptrace"
)

func main() {
	argv := os.Args[1:]
	if len(argv) > 0 && argv[0] == "--" {
		argv = argv[1:]
	}
	if len(argv) == 0 {
		log.Fatal("not enough arguments after --")
	}

	nginxConfigPath := getNginxConfByArg("-c", argv)
	if nginxConfigPath == "" {
		log.Fatal("nginx config not found in args")
	}

	realPathNginxConf, err := filepath.EvalSymlinks(nginxConfigPath)
	if err != nil {
		log.Fatal("can't eval real config path for nginx config: %w", err)
	}

	extraRead := []string{
		"/etc/nginx/",
		"/usr/share/nginx/",
		realPathNginxConf,
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatal("Failed get pwd", err)
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
		os.Exit(1)
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

	if res.Error != "" || res.ExitStatus != 0 {
		log.Printf("error run: %v", res.Error)
	}
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
