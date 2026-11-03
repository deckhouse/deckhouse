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

package ipv4_options_guard

import "testing"

func TestDecisionModes(t *testing.T) {
	tests := []struct {
		name        string
		d           decision
		wantDefault string
		wantMode    string
	}{
		{"new cluster", decision{}, ModeEnforce, ModeEnforce},
		{"existing cluster: a CNI agent is running", decision{Existing: true}, ModeAudit, ModeAudit},
		{"agent appeared before the ConfigMap was rendered", decision{Pending: ModeEnforce, Existing: true}, ModeEnforce, ModeEnforce},
		{"decision kept in the ConfigMap wins", decision{Recorded: ModeEnforce, Pending: ModeAudit, Existing: true}, ModeEnforce, ModeEnforce},
		{"invalid value in the ConfigMap is ignored", decision{Recorded: "Bogus", Existing: true}, ModeAudit, ModeAudit},
		{"mode set in the module settings", decision{Configured: "Disabled"}, ModeEnforce, "Disabled"},
		{"settings win over the recorded default", decision{Recorded: ModeAudit, Configured: ModeEnforce}, ModeAudit, ModeEnforce},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDefault, gotMode := tt.d.modes()
			if gotDefault != tt.wantDefault || gotMode != tt.wantMode {
				t.Errorf("modes() = %s, %s; want %s, %s", gotDefault, gotMode, tt.wantDefault, tt.wantMode)
			}
		})
	}
}
