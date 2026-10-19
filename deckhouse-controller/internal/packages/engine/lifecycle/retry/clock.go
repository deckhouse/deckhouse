// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import "time"

// Clock is the time the package Reconciler, Current and Global decide and wait with: backoff
// deadlines, the update window, their wake timers. Tests drive it by hand.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

// Timer is a timer of a Clock (time.Timer semantics since Go 1.23: Reset and Stop need no drain).
type Timer interface {
	C() <-chan time.Time
	Reset(d time.Duration) bool
	Stop() bool
}

// Wall is the real clock, the default of every Deps.
type Wall struct{}

func (Wall) Now() time.Time { return time.Now() }

func (Wall) NewTimer(d time.Duration) Timer { return wallTimer{time.NewTimer(d)} }

type wallTimer struct{ *time.Timer }

func (t wallTimer) C() <-chan time.Time { return t.Timer.C }

// OrWall returns c, or Wall when c is nil.
func OrWall(c Clock) Clock {
	if c == nil {
		return Wall{}
	}

	return c
}
