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

package providercheck

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestIsNetworkUnreachable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "generic", err: errors.New("invalid_client"), want: false},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{
			name: "dns",
			err:  &net.DNSError{Err: "no such host", Name: "api.github.com", IsNotFound: true},
			want: true,
		},
		{
			name: "url wrapped timeout",
			err:  &url.Error{Op: "Get", URL: "https://api.github.com/meta", Err: context.DeadlineExceeded},
			want: true,
		},
		{
			name: "connection refused text",
			err:  errors.New("dial tcp: connection refused"),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isNetworkUnreachable(tt.err); got != tt.want {
				t.Errorf("isNetworkUnreachable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestFailUnreachableAddsHint(t *testing.T) {
	t.Parallel()

	result := &dexProviderCheckResult{checks: []DexProviderCheckStepStatus{}}
	err := context.DeadlineExceeded
	result.failUnreachable("githubAPI", err, "URL %q is not reachable: %v", "https://api.github.com/meta", err)
	if len(result.checks) != 1 {
		t.Fatalf("checks = %d, want 1", len(result.checks))
	}
	if !strings.Contains(result.checks[0].Message, unreachableHint) {
		t.Fatalf("missing closed-contour hint: %q", result.checks[0].Message)
	}

	result = &dexProviderCheckResult{checks: []DexProviderCheckStepStatus{}}
	result.failUnreachable("githubCredentials", errors.New("invalid_client"), "GitHub rejected the client credentials")
	if strings.Contains(result.checks[0].Message, unreachableHint) {
		t.Fatalf("hint on non-network error: %q", result.checks[0].Message)
	}
}
