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

package bashible

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreflightCheck(t *testing.T) {
	tests := []struct {
		name        string
		input       Inputs
		expected    Result
		msgContains []string
	}{
		{
			name: "All nodes ready (default config)",
			input: Inputs{
				IsSecretExist: false,
				NodeStatus: map[string]InputsNodeStatus{
					"node-1": {ContainerdCfgMode: containerdCfgModeDefault},
					"node-2": {ContainerdCfgMode: containerdCfgModeDefault},
				},
			},
			expected: Result{Ready: true},
			msgContains: []string{
				preflightCheckMessage,
				"All 2 node(s) Ready to configure.",
			},
		},
		{
			name: "Secret exists, all nodes considered ready",
			input: Inputs{
				IsSecretExist: true,
				NodeStatus: map[string]InputsNodeStatus{
					"node-1": {ContainerdCfgMode: containerdCfgModeCustom},
					"node-2": {ContainerdCfgMode: "unknown"},
				},
			},
			expected: Result{Ready: true},
			msgContains: []string{
				preflightCheckMessage,
				"Configuration from registry module already exists.",
				"All 2 node(s) Ready to configure.",
			},
		},
		{
			name: "Some nodes unready (custom config)",
			input: Inputs{
				IsSecretExist: false,
				NodeStatus: map[string]InputsNodeStatus{
					"node-1": {ContainerdCfgMode: containerdCfgModeDefault},
					"node-2": {ContainerdCfgMode: containerdCfgModeCustom},
					"node-3": {ContainerdCfgMode: containerdCfgModeDefault},
				},
			},
			expected: Result{Ready: false},
			msgContains: []string{
				preflightCheckMessage,
				"1/3 node(s) Unready:",
				"- node-2: has custom toml merge containerd configuration",
			},
		},
		{
			name: "Some nodes unready (unknown config)",
			input: Inputs{
				IsSecretExist: false,
				NodeStatus: map[string]InputsNodeStatus{
					"node-1": {ContainerdCfgMode: ""},
					"node-2": {ContainerdCfgMode: containerdCfgModeDefault},
				},
			},
			expected: Result{Ready: false},
			msgContains: []string{
				preflightCheckMessage,
				"1/2 node(s) Unready:",
				"- node-1: unknown containerd configuration, waiting...",
			},
		},
		{
			name: "All nodes unready (mixed reasons)",
			input: Inputs{
				IsSecretExist: false,
				NodeStatus: map[string]InputsNodeStatus{
					"node-1": {ContainerdCfgMode: containerdCfgModeCustom},
					"node-2": {ContainerdCfgMode: "some-other-mode"},
				},
			},
			expected: Result{Ready: false},
			msgContains: []string{
				preflightCheckMessage,
				"2/2 node(s) Unready:",
				"- node-1: has custom toml merge containerd configuration",
				"- node-2: unknown containerd configuration, waiting...",
			},
		},
		{
			name: "No nodes",
			input: Inputs{
				IsSecretExist: false,
				NodeStatus:    map[string]InputsNodeStatus{},
			},
			expected: Result{Ready: true},
			msgContains: []string{
				preflightCheckMessage,
				"All 0 node(s) Ready to configure.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreflightCheck(tt.input)
			assert.Equal(t, tt.expected.Ready, result.Ready)

			for _, msg := range tt.msgContains {
				assert.True(t, strings.Contains(result.Message, msg), "Expected message to contain %q, but got %q", msg, result.Message)
			}
		})
	}
}
