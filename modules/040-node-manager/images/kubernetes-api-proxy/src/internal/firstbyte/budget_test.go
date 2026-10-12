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

package firstbyte

import (
	"testing"
	"time"
)

func TestBudget_Wait(t *testing.T) {
	const base = 100 * time.Millisecond

	tests := []struct {
		name string
		run  func(b *Budget)
		want time.Duration
	}{
		{
			name: "unknown addr returns floor",
			run:  func(*Budget) {},
			want: base,
		},
		{
			name: "observe doubles the latency",
			run:  func(b *Budget) { b.Observe("a", 75*time.Millisecond) },
			want: 150 * time.Millisecond,
		},
		{
			name: "observe clamps at twice the floor",
			run:  func(b *Budget) { b.Observe("a", 2*base) },
			want: 2 * base,
		},
		{
			name: "observe clamps at the floor",
			run:  func(b *Budget) { b.Observe("a", base/10) },
			want: base,
		},
		{
			name: "timeout grows by half",
			run:  func(b *Budget) { b.Timeout("a") },
			want: 150 * time.Millisecond,
		},
		{
			name: "timeout caps at twice the floor",
			run: func(b *Budget) {
				b.Timeout("a")
				b.Timeout("a")
				b.Timeout("a")
			},
			want: 2 * base,
		},
		{
			name: "addrs are independent",
			run:  func(b *Budget) { b.Observe("b", 2*base) },
			want: base,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(base)
			tt.run(b)

			if got := b.Wait("a"); got != tt.want {
				t.Errorf("Wait() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestBudget_Chain(t *testing.T) {
	const base = 100 * time.Millisecond

	b := New(base)
	b.Observe("a", 2*base)
	b.Timeout("a")

	if got := b.Chain(); got != 3*base {
		t.Errorf("Chain() = %s, want %s (three floors, independent of per-addr growth)", got, 3*base)
	}
}
