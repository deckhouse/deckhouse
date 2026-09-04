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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestRetrySetup(t *testing.T) {
	t.Parallel()

	t.Run("succeeds after transient failures", func(t *testing.T) {
		t.Parallel()
		calls := 0
		_, err := retrySetup(t.Context(), logr.Discard(), time.Minute, time.Millisecond, func() (manager.Manager, error) {
			calls++
			if calls < 3 {
				return nil, errors.New("api server unreachable")
			}
			return nil, nil
		})
		if err != nil || calls != 3 {
			t.Fatalf("err = %v, calls = %d, want success on the third call", err, calls)
		}
	})

	t.Run("gives up after the window", func(t *testing.T) {
		t.Parallel()
		_, err := retrySetup(t.Context(), logr.Discard(), 10*time.Millisecond, time.Millisecond, func() (manager.Manager, error) {
			return nil, errors.New("still down")
		})
		if err == nil || err.Error() != "still down" {
			t.Fatalf("err = %v, want the last setup error", err)
		}
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := retrySetup(ctx, logr.Discard(), time.Minute, time.Minute, func() (manager.Manager, error) {
			return nil, errors.New("down")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}

func TestLogLevel(t *testing.T) {
	cases := map[string]zapcore.Level{"": zapcore.InfoLevel, "debug": zapcore.DebugLevel, "warn": zapcore.WarnLevel, "ERROR": zapcore.ErrorLevel, "loud": zapcore.InfoLevel}
	for in, want := range cases {
		if got := logLevel(in); got != want {
			t.Errorf("logLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

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
	if !opts.LeaderElection || !opts.LeaderElectionReleaseOnCancel || opts.LeaderElectionID != controllerName || opts.LeaderElectionNamespace != leaderElectionNamespace {
		t.Fatalf("leader election must be on in the module namespace, got %+v", opts)
	}
	if opts.Cache.DefaultTransform == nil {
		t.Error("cached objects must be stripped of managedFields")
	}
}
