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
		"--metrics-bind-address=127.0.0.1:9441",
		"--leader-elect=true",
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.MetricsBindAddress != "127.0.0.1:9441" {
		t.Errorf("bind address not taken from flags: %+v", cfg)
	}

	if !cfg.LeaderElection || cfg.Namespace != namespace {
		t.Errorf("leader election not configured from flag and environment: %+v", cfg)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q, want %q", cfg.LogLevel, "debug")
	}
}

// The namespace holds the machines, so it is necessary also without leader
// election.
func TestLoadRequiresNamespace(t *testing.T) {
	for name, args := range map[string][]string{
		"with leader election":    {"--leader-elect=true"},
		"without leader election": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(common.EnvPodNamespace, "")

			if _, err := Load(args); err == nil {
				t.Fatalf("expected an error when %s is empty", common.EnvPodNamespace)
			}
		})
	}
}

func TestLoadAcceptsDisabledListeners(t *testing.T) {
	t.Setenv(common.EnvPodNamespace, namespace)

	cfg, err := Load([]string{"--metrics-bind-address=0"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.MetricsBindAddress != "0" {
		t.Errorf("disabled listener was rewritten: %+v", cfg)
	}
}

func TestLoadRejectsInvalidInput(t *testing.T) {
	for name, tc := range map[string]struct {
		args     []string
		logLevel string
	}{
		"metrics address without a port": {args: []string{"--metrics-bind-address=localhost"}},
		"empty metrics address":          {args: []string{"--metrics-bind-address="}},
		"unknown flag":                   {args: []string{"--unknown-flag=1s"}},
		"unparsable log level":           {logLevel: "verbose"},
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
