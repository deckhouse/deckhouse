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

package api

import (
	"crypto/sha256"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// A user may sign out everywhere or ask for a password change once in 10 seconds, three times
	// in a row at most. Both endpoints share the budget.
	userRequestInterval = 10 * time.Second
	userRequestBurst    = 3

	// userLimiterSweepInterval is how often the limiters of idle users are dropped.
	userLimiterSweepInterval = time.Minute
)

// userLimiter limits the requests of every user, keyed by SHA-256 of the sub claim of their token.
type userLimiter struct {
	mu        sync.Mutex
	limiters  map[[sha256.Size]byte]*userLimit
	interval  time.Duration
	burst     int
	lastSweep time.Time
}

type userLimit struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newUserLimiter(interval time.Duration, burst int) *userLimiter {
	return &userLimiter{
		limiters: make(map[[sha256.Size]byte]*userLimit),
		interval: interval,
		burst:    burst,
	}
}

// reserve takes a request from the budget of the subject at now. It returns zero when the request
// may go on, or how long the user has to wait otherwise; a refused request takes nothing.
func (l *userLimiter) reserve(subject string, now time.Time) time.Duration {
	key := sha256.Sum256([]byte(subject))

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	limit, ok := l.limiters[key]
	if !ok {
		limit = &userLimit{limiter: rate.NewLimiter(rate.Every(l.interval), l.burst)}
		l.limiters[key] = limit
	}
	limit.lastSeen = now

	reservation := limit.limiter.ReserveN(now, 1)
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return delay
	}
	return 0
}

// sweep drops the limiters of users idle long enough to have their whole budget back: a new
// limiter is the same as theirs, so the map only holds users who are being limited.
func (l *userLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < userLimiterSweepInterval {
		return
	}
	l.lastSweep = now

	refill := l.interval * time.Duration(l.burst)
	for key, limit := range l.limiters {
		if now.Sub(limit.lastSeen) >= refill {
			delete(l.limiters, key)
		}
	}
}
