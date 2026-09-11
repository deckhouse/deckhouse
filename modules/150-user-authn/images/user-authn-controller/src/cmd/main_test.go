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
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewManagerOptions(t *testing.T) {
	tests := []struct {
		name                 string
		haMode               string
		wantLeaderElection   bool
		wantLeaderElectionID string
		wantLeaderElectionNS string
	}{
		// Leader election does not depend on HA_MODE any more: without HA the Deployment keeps its
		// default rolling update, so two pods run during every version change, and two of them
		// writing the same UserAccounts and DexProviderChecks would fight over them.
		{
			name:                 "ha mode unset",
			haMode:               "",
			wantLeaderElection:   true,
			wantLeaderElectionID: controllerName,
			wantLeaderElectionNS: leaderElectionNamespace,
		},
		{
			name:                 "ha mode false",
			haMode:               "false",
			wantLeaderElection:   true,
			wantLeaderElectionID: controllerName,
			wantLeaderElectionNS: leaderElectionNamespace,
		},
		{
			name:                 "ha mode true",
			haMode:               "true",
			wantLeaderElection:   true,
			wantLeaderElectionID: controllerName,
			wantLeaderElectionNS: leaderElectionNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(haModeEnv, tt.haMode)

			opts := newManagerOptions(runtime.NewScheme())

			if opts.LeaderElection != tt.wantLeaderElection {
				t.Errorf("LeaderElection = %v, want %v", opts.LeaderElection, tt.wantLeaderElection)
			}
			if !opts.LeaderElectionReleaseOnCancel {
				t.Error("the lease must be released on shutdown, or the next pod waits out the lease duration")
			}
			if opts.LeaderElectionID != tt.wantLeaderElectionID {
				t.Errorf("LeaderElectionID = %q, want %q", opts.LeaderElectionID, tt.wantLeaderElectionID)
			}
			if opts.LeaderElectionNamespace != tt.wantLeaderElectionNS {
				t.Errorf("LeaderElectionNamespace = %q, want %q", opts.LeaderElectionNamespace, tt.wantLeaderElectionNS)
			}
			if opts.HealthProbeBindAddress != ":9090" {
				t.Errorf("HealthProbeBindAddress = %q, want %q", opts.HealthProbeBindAddress, ":9090")
			}
			if opts.Metrics.BindAddress != ":9091" {
				t.Errorf("Metrics.BindAddress = %q, want %q", opts.Metrics.BindAddress, ":9091")
			}
			if opts.WebhookServer != nil {
				t.Errorf("WebhookServer = %v, want nil", opts.WebhookServer)
			}
		})
	}
}
