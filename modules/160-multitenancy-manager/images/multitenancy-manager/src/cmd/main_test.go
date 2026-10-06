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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestManagerOptions(t *testing.T) {
	// The options must not depend on the replica count; HA_MODE, the variable that would carry it,
	// changes nothing.
	tests := []struct {
		name   string
		haMode string
	}{
		{name: "ha installation", haMode: "true"},
		{name: "single replica", haMode: "false"},
		{name: "no ha mode", haMode: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HA_MODE", tt.haMode)
			is := assert.New(t)
			must := require.New(t)
			scheme := runtime.NewScheme()

			opts := managerOptions(scheme)

			// The lease key is spelled out rather than taken from the constants: every version of the
			// controller has to contend for this very Lease, or two of them write Helm releases at once.
			is.True(opts.LeaderElection)
			is.Empty(opts.LeaderElectionResourceLock, "the default lock is a Lease")
			is.Equal("multitenancy-manager", opts.LeaderElectionID)
			is.Equal("d8-multitenancy-manager", opts.LeaderElectionNamespace)
			is.True(opts.LeaderElectionReleaseOnCancel)
			is.Same(scheme, opts.Scheme)

			must.NotNil(opts.LeaseDuration)
			must.NotNil(opts.RenewDeadline)
			must.NotNil(opts.RetryPeriod)
			is.Equal(60*time.Second, *opts.LeaseDuration)
			is.Equal(40*time.Second, *opts.RenewDeadline)
			is.Equal(8*time.Second, *opts.RetryPeriod)
		})
	}
}
