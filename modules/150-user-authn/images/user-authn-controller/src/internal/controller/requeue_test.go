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

package controller

import (
	"testing"
	"time"
)

func TestRequeueAfterTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * time.Second)
	past := now.Add(-time.Second)

	tests := []struct {
		name  string
		until *time.Time
		want  time.Duration
	}{
		{name: "nil until", want: 0},
		{name: "expired", until: &past, want: 0},
		{name: "active", until: &future, want: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RequeueAfterTime(tt.until, now)
			if got != tt.want {
				t.Errorf("RequeueAfterTime() = %v, want %v", got, tt.want)
			}
		})
	}
}
