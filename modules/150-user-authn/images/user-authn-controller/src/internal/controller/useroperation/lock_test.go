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

package useroperation

import (
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestResolveLockUntil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "permanent sentinel resolves to the year-9999 lock-forever marker",
			input: lockForever,
			want:  foreverTime,
		},
		{
			name:  "plain Go duration is added to now",
			input: "30m",
			want:  fixedNow.Add(30 * time.Minute),
		},
		{
			name:  "compound Go duration with no days unit",
			input: "2h30m",
			want:  fixedNow.Add(2*time.Hour + 30*time.Minute),
		},
		{
			name:  "single days segment expands to 24h-per-day",
			input: "7d",
			want:  fixedNow.Add(7 * 24 * time.Hour),
		},
		{
			name:  "fractional days are honoured",
			input: "0.5d",
			want:  fixedNow.Add(12 * time.Hour),
		},
		{
			name:  "days mix freely with other Go-duration units",
			input: "1d12h",
			want:  fixedNow.Add(36 * time.Hour),
		},
		{
			name:    "non-parseable garbage surfaces an error",
			input:   "never",
			wantErr: true,
		},
		{
			name:    "explicitly zero duration is rejected as non-positive",
			input:   "0s",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveLockUntil(tt.input, fixedNow)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveLockUntil(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("resolveLockUntil(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
