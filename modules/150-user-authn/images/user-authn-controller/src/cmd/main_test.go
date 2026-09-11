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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewManagerOptions(t *testing.T) {
	// Leader election does not depend on HA any more: without HA the Deployment keeps its default
	// rolling update, so two pods run during every version change, and two of them writing the
	// same UserAccounts and DexProviderChecks would fight over them.
	opts := newManagerOptions(runtime.NewScheme())

	if !opts.LeaderElection || opts.LeaderElectionID != controllerName || opts.LeaderElectionNamespace != leaderElectionNamespace {
		t.Fatalf("leader election must be on in the module namespace, got %+v", opts)
	}
	if !opts.LeaderElectionReleaseOnCancel {
		t.Error("the lease must be released on shutdown, or the next pod waits out the lease duration")
	}
	// Losing the lease exits the process, so the lease has to outlive a short API-server absence.
	if opts.LeaseDuration == nil || *opts.LeaseDuration < 30*time.Second {
		t.Errorf("LeaseDuration = %v, want at least 30s", opts.LeaseDuration)
	}
	if opts.RenewDeadline == nil || opts.LeaseDuration == nil || *opts.RenewDeadline >= *opts.LeaseDuration {
		t.Errorf("RenewDeadline = %v must be shorter than LeaseDuration = %v", opts.RenewDeadline, opts.LeaseDuration)
	}
	if opts.RetryPeriod == nil || opts.RenewDeadline == nil || *opts.RetryPeriod*2 > *opts.RenewDeadline {
		t.Errorf("RetryPeriod = %v must leave several tries within RenewDeadline = %v", opts.RetryPeriod, opts.RenewDeadline)
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
}
