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

package checks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The checks that upload a script and read what it says.
//
// The rendered script is a temp file named after the output, not after the template, which is why
// onScript matches "check_ports.sh" and candiOptionsFor writes "check_ports.sh.tpl".
//
// All three report the same way, through scriptFailure: what the script printed outranks the exit
// status, which outranks the transport error. The order matters — the script's own text is the
// only one of the three written for an operator — and it is the order the old code got wrong on
// the default backend, where the exit-status branch was unreachable and every failure fell
// through to "Could not execute a script to …".

// TestDeckhouseUserOverTheDefaultBackend covers the check that used to lose the script's message.
func TestDeckhouseUserOverTheDefaultBackend(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "no conflicting user",
			node: newFakeNode().onScript("check_deckhouse_user.sh").
				prints("deckhouse user and group are not present\n"),
		},
		{
			// What the script said, which names the conflict.
			name: "the user already exists",
			node: newFakeNode().onScript("check_deckhouse_user.sh").
				printsAndExits("user deckhouse already exists on the node", 1),
			wantErr: "user deckhouse already exists on the node on",
		},
		{
			// A non-zero status with nothing printed: the status is all there is to report.
			name:    "it failed silently",
			node:    newFakeNode().onScript("check_deckhouse_user.sh").exits(3),
			wantErr: "script exited with status 3 on",
		},
		{
			// It never ran at all.
			name: "the session died",
			node: newFakeNode().onScript("check_deckhouse_user.sh").
				fails(errors.New("ssh: connection lost")),
			wantErr: "cannot check the deckhouse user and group on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := DeckhouseUserCheck{
				NodeInterface: FixedNodeInterface(tt.node),
				globalOptions: candiOptionsFor(t, "check_deckhouse_user.sh.tpl"),
			}

			err := check.Run(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLocalhostResolutionOverTheDefaultBackend(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "localhost resolves",
			node: newFakeNode().onScript("check_localhost.sh").succeeds(),
		},
		{
			name: "it does not",
			node: newFakeNode().onScript("check_localhost.sh").
				printsAndExits("localhost does not resolve to 127.0.0.1", 1),
			wantErr: "localhost does not resolve to 127.0.0.1 on",
		},
		{
			name:    "the script could not be run",
			node:    newFakeNode().onScript("check_localhost.sh").fails(errors.New("permission denied")),
			wantErr: "cannot check that localhost resolves on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := LocalhostDomainCheck{
				NodeInterface: FixedNodeInterface(tt.node),
				globalOptions: candiOptionsFor(t, "check_localhost.sh.tpl"),
			}

			err := check.Run(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestPortsOverTheDefaultBackend(t *testing.T) {
	tests := []struct {
		name    string
		node    *fakeNode
		wantErr string
	}{
		{
			name: "the ports are free",
			node: newFakeNode().onScript("check_ports.sh").succeeds(),
		},
		{
			name: "one is taken",
			node: newFakeNode().onScript("check_ports.sh").
				printsAndExits("port 6443 is already in use", 1),
			wantErr: "port 6443 is already in use on",
		},
		{
			name:    "the script could not be run",
			node:    newFakeNode().onScript("check_ports.sh").fails(errors.New("ssh: connection lost")),
			wantErr: "cannot check that the required ports are free on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAvailabilityPorts(t.Context(), tt.node, candiOptionsFor(t, "check_ports.sh.tpl"))

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPythonModulesOverTheDefaultBackend: the modules are probed by running the interpreter, and
// a module that is absent makes it exit non-zero — which on the default backend is an
// *ssh.ExitError. The check reads that status to tell "the module is missing" from "the command
// could not be run", and it used to read it from a type the backend never produces.
func TestPythonModulesOverTheDefaultBackend(t *testing.T) {
	// A node with python3 and every module the bootstrap scripts import.
	complete := func() *fakeNode {
		node := newFakeNode().on("command -v python3").prints("/usr/bin/python3")
		for _, module := range []string{
			"urllib.request", "urllib.error", "configparser", "http.server",
		} {
			node = node.on("python3 -c import " + module).succeeds()
		}
		return node
	}

	t.Run("everything is there", func(t *testing.T) {
		check := PythonCheck{NodeInterface: FixedNodeInterface(complete())}

		detail, err := check.Run(t.Context())
		require.NoError(t, err)
		assert.Contains(t, detail, "python3 on")
	})

	t.Run("no interpreter at all", func(t *testing.T) {
		// The default answer is exit 127 for every `command -v`, so none of the three names is
		// on PATH.
		check := PythonCheck{NodeInterface: FixedNodeInterface(newFakeNode())}

		_, err := check.Run(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "none of them is on PATH")
	})

	t.Run("a module is missing", func(t *testing.T) {
		// configparser absent, and its Python 2 alternative too.
		node := complete().
			on("python3 -c import configparser").exits(1).
			on("python3 -c import ConfigParser").exits(1)

		check := PythonCheck{NodeInterface: FixedNodeInterface(node)}

		_, err := check.Run(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configparser or ConfigParser")
	})

	t.Run("every missing module is reported", func(t *testing.T) {
		// They are installed together, so reporting them one per run costs a bootstrap attempt
		// each.
		node := newFakeNode().on("command -v python3").prints("/usr/bin/python3")

		check := PythonCheck{NodeInterface: FixedNodeInterface(node)}

		_, err := check.Run(t.Context())
		require.Error(t, err)
		for _, want := range []string{"urllib.request or urllib2", "configparser or ConfigParser"} {
			assert.Contains(t, err.Error(), want)
		}
	})

	t.Run("the interpreter could not be run", func(t *testing.T) {
		node := complete().on("python3 -c import urllib.request").fails(errors.New("ssh: connection lost"))

		check := PythonCheck{NodeInterface: FixedNodeInterface(node)}

		_, err := check.Run(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot check the python modules on")
	})
}
