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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// Losing the lease exits the process, so the lease has to outlive a short API-server absence.
	if opts.LeaseDuration == nil || *opts.LeaseDuration < 30*time.Second {
		t.Errorf("LeaseDuration = %v, want at least 30s", opts.LeaseDuration)
	}
	if opts.RenewDeadline == nil || *opts.RenewDeadline >= *opts.LeaseDuration {
		t.Errorf("RenewDeadline = %v must be shorter than LeaseDuration = %v", opts.RenewDeadline, opts.LeaseDuration)
	}
	if opts.RetryPeriod == nil || *opts.RetryPeriod*2 > *opts.RenewDeadline {
		t.Errorf("RetryPeriod = %v must leave several tries within RenewDeadline = %v", opts.RetryPeriod, opts.RenewDeadline)
	}
}

// stubReader is a client.Reader whose List answers with a fixed error.
type stubReader struct {
	client.Reader
	err error
}

func (s stubReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return s.err
}

// Readiness has to say whether the controller can work now, not whether it warmed up once. With
// its RBAC taken away the controller created no bindings for a new rule and logged nothing, while
// both probes kept answering 200: the informers had synced before the loss and never un-sync.
func TestAPIAccessCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	if err := apiAccessCheck(stubReader{})(req); err != nil {
		t.Fatalf("a reader that answers: %v", err)
	}

	forbidden := errors.New(`clusterauthorizationrules.deckhouse.io is forbidden`)
	err := apiAccessCheck(stubReader{err: forbidden})(req)
	if err == nil {
		t.Fatal("a reader that is forbidden: want not ready")
	}
	if !errors.Is(err, forbidden) {
		t.Errorf("the cause must be kept for the probe body, got %v", err)
	}
}

// The check must not outlive the probe: the kubelet gives it three seconds.
func TestAPIAccessCheckHonoursTheDeadline(t *testing.T) {
	slow := stubReaderFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	start := time.Now()
	if err := apiAccessCheck(slow)(req); err == nil {
		t.Fatal("a reader that hangs: want not ready")
	}
	if took := time.Since(start); took > apiAccessCheckTimeout+time.Second {
		t.Errorf("the check waited %v, must give up after %v", took, apiAccessCheckTimeout)
	}
}

type stubReaderFunc func(ctx context.Context) error

func (f stubReaderFunc) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return nil
}
func (f stubReaderFunc) List(ctx context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return f(ctx)
}
