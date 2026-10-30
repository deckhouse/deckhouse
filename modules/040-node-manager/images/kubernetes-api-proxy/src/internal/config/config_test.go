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

package config

import (
	"flag"
	"os"
	"testing"
	"time"
)

// Without flags the proxy probes with the defaults, and fail-step is on.
func TestParse_ProbeDefaults(t *testing.T) {
	args, flags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, flags })

	os.Args = []string{"kubernetes-api-proxy"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	cfg := Parse()

	if cfg.HealthInterval != time.Second || cfg.HealthTimeout != 500*time.Millisecond || cfg.HealthJitter != 0.2 {
		t.Errorf("interval, timeout, jitter = %v, %v, %v, want 1s, 500ms, 0.2", cfg.HealthInterval, cfg.HealthTimeout, cfg.HealthJitter)
	}

	if cfg.FailStepInterval != 500*time.Millisecond || cfg.FailStepWindow != 30*time.Second {
		t.Errorf("fail-step interval, window = %v, %v, want 500ms, 30s", cfg.FailStepInterval, cfg.FailStepWindow)
	}
}

// The fail-step flags set the fail-step interval and window.
func TestParse_FailStepFlags(t *testing.T) {
	args, flags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, flags })

	os.Args = []string{"kubernetes-api-proxy", "--fail-step-interval=300ms", "--fail-step-window=10s"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	cfg := Parse()

	if cfg.FailStepInterval != 300*time.Millisecond || cfg.FailStepWindow != 10*time.Second {
		t.Errorf("fail-step interval, window = %v, %v, want 300ms, 10s", cfg.FailStepInterval, cfg.FailStepWindow)
	}
}

// Without flags the health endpoints are on 127.0.0.1:6480: the address that
// the static pod manifest sets.
func TestParse_HealthListenDefault(t *testing.T) {
	args, flags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, flags })
	t.Setenv("HEALTH_LISTEN", "")

	os.Args = []string{"kubernetes-api-proxy"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	if got := Parse().ProxyHealthListen; got != "127.0.0.1:6480" {
		t.Errorf("health listen address = %q, want 127.0.0.1:6480", got)
	}
}

// Without flags the proxy serves for 3s after a stop signal. The delay and the
// shutdown of the health server stay below the 5s that kubelet gives to
// node-critical pods on a worker shutdown.
func TestParse_ShutdownDelayDefault(t *testing.T) {
	args, flags := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, flags })

	os.Args = []string{"kubernetes-api-proxy"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	if got := Parse().ShutdownDelay; got != 3*time.Second {
		t.Errorf("shutdown delay = %v, want 3s", got)
	}
}
