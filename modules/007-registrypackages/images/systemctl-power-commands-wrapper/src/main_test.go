/*
Copyright 2025 Flant JSC

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
	"os"
	"testing"
)

func TestParseArgsHalt(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"halt"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if config.action != ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", config.action)
	}
}

func TestParseArgsPoweroff(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"poweroff"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if config.action != ActionPoweroff {
		t.Errorf("Expected ActionPoweroff, got %v", config.action)
	}
}

func TestParseArgsReboot(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"reboot"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if config.action != ActionReboot {
		t.Errorf("Expected ActionReboot, got %v", config.action)
	}
}

func TestParseArgsShutdown(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"shutdown"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if config.action != ActionPoweroff {
		t.Errorf("Expected ActionPoweroff for shutdown, got %v", config.action)
	}
}

func TestParseArgsDryRun(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"reboot", "--dry-run"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if !config.dryRun {
		t.Errorf("Expected dryRun to be true")
	}
}

func TestParseArgsRebootFlag(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"shutdown", "-r"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if config.action != ActionReboot {
		t.Errorf("Expected ActionReboot with -r flag, got %v", config.action)
	}
}

func TestParseArgsHelp(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"unknown-command"}
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if !config.help {
		t.Errorf("Expected help to be true for unknown command")
	}
}

func TestActionString(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionHalt, "halt"},
		{ActionPoweroff, "poweroff"},
		{ActionReboot, "reboot"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expected {
			t.Errorf("Action.String() = %v, want %v", got, tt.expected)
		}
	}
}
