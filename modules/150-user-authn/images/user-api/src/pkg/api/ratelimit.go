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

	"user-api/pkg/auth"
)

const (
	// A device of a user may act on the account (sign out everywhere or on devices, ask for a
	// password change) once in 10 seconds, three times in a row at most. The actions share the
	// budget.
	userRequestInterval = 10 * time.Second
	userRequestBurst    = 3

	// A device of a user may read the list of the devices once in 2 seconds, five times in a row
	// at most.
	userReadInterval = 2 * time.Second
	userReadBurst    = 5

	// userLimiterSweepInterval is how often the limiters of idle devices are dropped.
	userLimiterSweepInterval = time.Minute
)

// userLimiter limits the requests of every device of a user, keyed by SHA-256 of budgetKey. Each
// device has a budget of its own: a device signed in a moment ago, which may not act on the
// others yet, cannot use up the budget of the owner's device and keep it from signing the new
// device out.
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

// budgetKey names the budget of the device of claims: the user and the browser session (sid) of the
// token, or else its refresh token (d8_gid), as currentDevice finds the device. Tokens that name no
// device share the budget of the user; they cannot act on the account anyway.
func budgetKey(claims *auth.Claims) string {
	switch {
	case claims.SessionID != "":
		return claims.Subject + "\x00sid\x00" + claims.SessionID
	case claims.GrantID != "":
		return claims.Subject + "\x00gid\x00" + claims.GrantID
	default:
		return claims.Subject
	}
}

// reserve takes a request from the budget of key at now. It returns zero when the request may go
// on, or how long the device has to wait otherwise; a refused request takes nothing.
func (l *userLimiter) reserve(key string, now time.Time) time.Duration {
	hashed := sha256.Sum256([]byte(key))

	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	limit, ok := l.limiters[hashed]
	if !ok {
		limit = &userLimit{limiter: rate.NewLimiter(rate.Every(l.interval), l.burst)}
		l.limiters[hashed] = limit
	}
	limit.lastSeen = now

	reservation := limit.limiter.ReserveN(now, 1)
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return delay
	}
	return 0
}

// sweep drops the limiters of devices idle long enough to have their whole budget back: a new
// limiter is the same as theirs, so the map only holds devices that are being limited.
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
