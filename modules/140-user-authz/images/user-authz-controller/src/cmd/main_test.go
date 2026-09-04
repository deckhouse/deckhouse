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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestEnvInt(t *testing.T) {
	cases := map[string]struct {
		value string
		want  int
	}{
		"unset":     {"", 8},
		"valid":     {"16", 16},
		"garbage":   {"sixteen", 8},
		"zero":      {"0", 8},
		"negative":  {"-4", 8},
		"fraction":  {"1.5", 8},
		"with_sign": {"+12", 12},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			const key = "USER_AUTHZ_TEST_ENV_INT"
			if tc.value != "" {
				t.Setenv(key, tc.value)
			}
			if got := envInt(logr.Discard(), key, 8); got != tc.want {
				t.Errorf("envInt(%q) = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestManagerOptionsAlwaysElectALeader(t *testing.T) {
	opts := newManagerOptions(runtime.NewScheme())
	if !opts.LeaderElection || opts.LeaderElectionID != controllerName || opts.LeaderElectionNamespace != leaderElectionNamespace {
		t.Fatalf("leader election must be on in the module namespace, got %+v", opts)
	}
	if opts.Cache.DefaultTransform == nil {
		t.Error("cached objects must be stripped of managedFields")
	}
}
