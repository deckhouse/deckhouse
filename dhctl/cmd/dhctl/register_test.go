// Copyright 2025 Flant JSC
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

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestCheckCommand(t *testing.T) {
	tests := []struct {
		name            string
		commandName     string
		allowedCommands []string
		expectedAllowed bool
		expectedSub     []string
	}{
		{
			name:            "empty allowed commands allows all",
			commandName:     "bootstrap",
			allowedCommands: []string{},
			expectedAllowed: true,
			expectedSub:     []string{},
		},
		{
			name:            "command in allowed list",
			commandName:     "bootstrap",
			allowedCommands: []string{"bootstrap", "converge"},
			expectedAllowed: true,
			expectedSub:     []string{},
		},
		{
			name:            "command not in allowed list",
			commandName:     "destroy",
			allowedCommands: []string{"bootstrap", "converge"},
			expectedAllowed: false,
			expectedSub:     []string{},
		},
		{
			name:            "command with subcommands",
			commandName:     "bootstrap",
			allowedCommands: []string{"bootstrap install", "converge"},
			expectedAllowed: true,
			expectedSub:     []string{"bootstrap", "install"},
		},
		{
			name:            "command with wildcard subcommands",
			commandName:     "bootstrap",
			allowedCommands: []string{"bootstrap *", "converge"},
			expectedAllowed: true,
			expectedSub:     []string{"bootstrap", "*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, sub := checkCommand(tt.commandName, tt.allowedCommands)
			require.Equal(t, tt.expectedAllowed, allowed)
			require.Equal(t, tt.expectedSub, sub)
		})
	}
}

func TestCheckSubcommand(t *testing.T) {
	tests := []struct {
		name        string
		subName     string
		subcommands []string
		expected    bool
	}{
		{
			name:        "wildcard allows all",
			subName:     "install",
			subcommands: []string{"bootstrap", "*"},
			expected:    true,
		},
		{
			name:        "exact match",
			subName:     "install",
			subcommands: []string{"bootstrap", "install"},
			expected:    true,
		},
		{
			name:        "no match",
			subName:     "destroy",
			subcommands: []string{"bootstrap", "install"},
			expected:    false,
		},
		{
			name:        "empty subcommands",
			subName:     "install",
			subcommands: []string{},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkSubcommand(tt.subName, tt.subcommands)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetParentIndex(t *testing.T) {
	commands := []Command{
		{Name: "bootstrap", Help: "Bootstrap cluster"},
		{Name: "converge", Help: "Converge cluster"},
		{Name: "install", Help: "Install component", Parrent: "bootstrap"},
	}

	tests := []struct {
		name        string
		parentName  string
		expectError bool
		expectedIdx int
	}{
		{
			name:        "existing parent",
			parentName:  "bootstrap",
			expectError: false,
			expectedIdx: 0,
		},
		{
			name:        "another existing parent",
			parentName:  "converge",
			expectError: false,
			expectedIdx: 1,
		},
		{
			name:        "non-existing parent",
			parentName:  "nonexistent",
			expectError: true,
			expectedIdx: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := getParentIndex(commands, tt.parentName)

			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, -1, idx)
				require.Contains(t, err.Error(), "not found in command list")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedIdx, idx)
			}
		})
	}
}

func TestGetNestingDepth(t *testing.T) {
	commands := []Command{
		{Name: "bootstrap", Help: "Bootstrap cluster"},
		{Name: "converge", Help: "Converge cluster"},
		{Name: "install", Help: "Install component", Parrent: "bootstrap"},
		{Name: "phase", Help: "Run phase", Parrent: "install"},
		{Name: "step", Help: "Run step", Parrent: "phase"},
	}

	tests := []struct {
		name          string
		cmd           Command
		expectedTop   string
		expectedDepth int
	}{
		{
			name:          "top level command",
			cmd:           Command{Name: "bootstrap", Help: "Bootstrap cluster"},
			expectedTop:   "bootstrap",
			expectedDepth: 0,
		},
		{
			name:          "one level deep",
			cmd:           Command{Name: "install", Help: "Install component", Parrent: "bootstrap"},
			expectedTop:   "bootstrap",
			expectedDepth: 1,
		},
		{
			name:          "two levels deep",
			cmd:           Command{Name: "phase", Help: "Run phase", Parrent: "install"},
			expectedTop:   "bootstrap",
			expectedDepth: 2,
		},
		{
			name:          "three levels deep",
			cmd:           Command{Name: "step", Help: "Run step", Parrent: "phase"},
			expectedTop:   "bootstrap",
			expectedDepth: 3,
		},
		{
			name:          "orphaned command",
			cmd:           Command{Name: "orphan", Help: "Orphaned command", Parrent: "nonexistent"},
			expectedTop:   "orphan",
			expectedDepth: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topLevel, depth := getNestingDepth(tt.cmd, commands)
			require.Equal(t, tt.expectedTop, topLevel.Name)
			require.Equal(t, tt.expectedDepth, depth)
		})
	}
}

func TestInitParent(t *testing.T) {
	// Create a test kingpin application
	app := kingpin.New("test", "Test application")

	// Create test command list
	testCommandList := []Command{
		{Name: "bootstrap", Help: "Bootstrap cluster", cmd: nil},
		{Name: "converge", Help: "Converge cluster", cmd: nil},
	}

	// Backup original commandList
	originalCommandList := commandList
	commandList = testCommandList
	defer func() {
		commandList = originalCommandList
	}()

	t.Run("initialize new parent command", func(t *testing.T) {
		pcmd := initParent(0, app)
		require.NotNil(t, pcmd)
		require.NotNil(t, commandList[0].cmd)
	})

	t.Run("return existing parent command", func(t *testing.T) {
		// First call should initialize
		pcmd1 := initParent(1, app)
		require.NotNil(t, pcmd1)
		require.NotNil(t, commandList[1].cmd)

		// Second call should return existing
		pcmd2 := initParent(1, app)
		require.NotNil(t, pcmd2)
		require.Equal(t, pcmd1, pcmd2)
	})
}

func TestRegisterCommands(t *testing.T) {
	tests := []struct {
		name            string
		commands        []Command
		allowedCommands []string
		expectError     bool
	}{
		{
			name: "register top level commands",
			commands: []Command{
				{Name: "bootstrap", Help: "Bootstrap cluster", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
				{Name: "converge", Help: "Converge cluster", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
			},
			allowedCommands: []string{},
			expectError:     false,
		},
		{
			name: "register nested commands",
			commands: []Command{
				{Name: "bootstrap", Help: "Bootstrap cluster", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
				{Name: "install", Help: "Install component", Parrent: "bootstrap", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
			},
			allowedCommands: []string{},
			expectError:     false,
		},
		{
			name: "error on missing parent",
			commands: []Command{
				{Name: "install", Help: "Install component", Parrent: "nonexistent", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
			},
			allowedCommands: []string{"nonexistent install"},
			expectError:     true,
		},
		{
			name: "filtered commands by allowed list",
			commands: []Command{
				{Name: "bootstrap", Help: "Bootstrap cluster", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
				{Name: "converge", Help: "Converge cluster", DefineFunc: func(cmd *kingpin.CmdClause) *kingpin.CmdClause { return cmd }},
			},
			allowedCommands: []string{"bootstrap"},
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test kingpin application
			app := kingpin.New("test", "Test application")

			// Backup original values
			originalCommandList := commandList
			originalAllowedCommands := allowedCommands

			// Set test values
			commandList = tt.commands
			allowedCommands = tt.allowedCommands

			defer func() {
				commandList = originalCommandList
				allowedCommands = originalAllowedCommands
			}()

			err := registerCommands(app)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
