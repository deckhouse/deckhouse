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
	"testing"

	"fencing-controller/internal/common"
)

const namespace = "d8-cloud-instance-manager"

func TestLoadWithoutFlagsMatchesDeploymentDefaults(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, namespace)

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.HealthProbeBindAddress != DefaultHealthProbeBindAddress {
		t.Errorf("health probe bind address = %q, want %q", cfg.HealthProbeBindAddress, DefaultHealthProbeBindAddress)
	}

	if cfg.MetricsBindAddress != DefaultMetricsBindAddress {
		t.Errorf("metrics bind address = %q, want %q", cfg.MetricsBindAddress, DefaultMetricsBindAddress)
	}

	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("log level = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}

	if cfg.LeaderElection {
		t.Error("leader election is on by default, want off unless --leader-elect is passed")
	}
}

func TestLoadReadsFlagsAndEnvironment(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, namespace)
	t.Setenv(common.EnvLogLevel, "debug")

	cfg, err := Load([]string{
		"--health-probe-bind-address=0.0.0.0:9440",
		"--metrics-bind-address=127.0.0.1:9441",
		"--leader-elect=true",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.HealthProbeBindAddress != "0.0.0.0:9440" || cfg.MetricsBindAddress != "127.0.0.1:9441" {
		t.Errorf("bind addresses not taken from flags: %+v", cfg)
	}

	if !cfg.LeaderElection || cfg.LeaderElectionNamespace != namespace {
		t.Errorf("leader election not configured from flag and environment: %+v", cfg)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadRequiresNamespaceWhenElecting(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, "")

	if _, err := Load([]string{"--leader-elect=true"}); err == nil {
		t.Fatalf("expected an error when %s is empty", common.EnvPodNamespace)
	}
}

func TestLoadWithoutElectionIgnoresNamespace(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, "")

	if _, err := Load(nil); err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestLoadAcceptsDisabledListeners(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, namespace)

	cfg, err := Load([]string{"--health-probe-bind-address=0", "--metrics-bind-address=0"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.HealthProbeBindAddress != "0" || cfg.MetricsBindAddress != "0" {
		t.Errorf("disabled listeners were rewritten: %+v", cfg)
	}
}

func TestLoadRejectsInvalidInput(t *testing.T) {
	for name, tc := range map[string]struct {
		args     []string
		logLevel string
	}{
		"health probe address without a port": {args: []string{"--health-probe-bind-address=0.0.0.0"}},
		"empty health probe address":          {args: []string{"--health-probe-bind-address="}},
		"metrics address without a port":      {args: []string{"--metrics-bind-address=localhost"}},
		"unknown flag":                        {args: []string{"--evacuation-delay=1s"}},
		"unparsable log level":                {logLevel: "verbose"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.EnvPodNamespace, namespace)
			t.Setenv(common.EnvLogLevel, tc.logLevel)

			if _, err := Load(tc.args); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
