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

package upstream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCalcTier(t *testing.T) {
	tests := []struct {
		name    string
		latency time.Duration
		want    Tier
	}{
		{"zero", 0, 0},
		{"1ms", time.Millisecond, 0},
		{"9ms", 9 * time.Millisecond, 0},
		{"10ms", 10 * time.Millisecond, 1},
		{"25ms", 25 * time.Millisecond, 2},
		{"50ms", 50 * time.Millisecond, 3},
		{"100ms", 100 * time.Millisecond, 4},
		{"250ms", 250 * time.Millisecond, 5},
		{"500ms", 500 * time.Millisecond, 6},
		{"10s", 10 * time.Second, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcTier(tt.latency); got != tt.want {
				t.Errorf("calcTier(%v) = %v, want %v", tt.latency, got, tt.want)
			}
		})
	}

	if tierFailed != Tier(len(tierBucket)) {
		t.Errorf("tierFailed = %v, want %v", tierFailed, len(tierBucket))
	}
}

func TestClassify(t *testing.T) {
	deadline := fmt.Errorf("do: %w", context.DeadlineExceeded)

	tests := []struct {
		name    string
		status  int
		err     error
		gotConn bool
		want    probeKind
	}{
		{name: "200", status: http.StatusOK, gotConn: true, want: probeReady},
		{name: "500", status: http.StatusInternalServerError, gotConn: true, want: probeNotReady},
		{name: "403", status: http.StatusForbidden, gotConn: true, want: probeNotReady},
		{name: "deadline on an open connection", err: deadline, gotConn: true, want: probeSoft},
		{name: "deadline before a connection", err: deadline, want: probeHard},
		{name: "refused", err: errors.New("connection refused"), want: probeHard},
		{name: "reset after connect", err: errors.New("connection reset"), gotConn: true, want: probeHard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.status, tt.err, tt.gotConn, time.Millisecond)
			if got.kind != tt.want {
				t.Errorf("classify().kind = %v, want %v", got.kind, tt.want)
			}

			hasLatency := tt.want == probeReady || tt.want == probeNotReady
			if hasLatency != (got.latency == time.Millisecond) {
				t.Errorf("latency = %v, want set only for answers", got.latency)
			}
		})
	}
}
