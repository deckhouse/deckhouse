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
	"testing"
	"time"
)

func TestCalcTier(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		elapsed  time.Duration
		expected Tier
		wantErr  bool
	}{
		{"Error returns -1", context.DeadlineExceeded, 10 * time.Millisecond, -1, true},
		{"0ms -> Tier 0", nil, 0, 0, false},
		{"1ms -> Tier 2", nil, 1 * time.Millisecond, 2, false},
		{"100ms -> Tier 4", nil, 100 * time.Millisecond, 4, false},
		{"1s -> Tier 7", nil, 1 * time.Second, 7, false},
		{"2s -> Tier 7", nil, 2 * time.Second, 7, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calcTier(tt.err, tt.elapsed)
			if (err != nil) != tt.wantErr {
				t.Errorf("calcTier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("calcTier() = %v, want %v", got, tt.expected)
			}
		})
	}
}
