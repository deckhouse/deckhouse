// Copyright 2024 Flant JSC
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

package requests_counter

import (
	"context"
	"sync"
	"time"
)

const (
	defaultCleanUpPeriod = time.Minute * 10
	defaultTTL           = time.Hour * 2
)

// RequestsCounter structure for tracking requests by methods
type RequestsCounter struct {
	mx             sync.Mutex
	requestsStore  map[string][]time.Time
	ttl            time.Duration
	cleanUpPreriod time.Duration
	taskSem        chan struct{}
}

// New constructor for RequestsCounter
func New(ttl time.Duration, taskSemChan chan struct{}) *RequestsCounter {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	at := &RequestsCounter{
		requestsStore:  make(map[string][]time.Time),
		ttl:            ttl,
		cleanUpPreriod: min(defaultCleanUpPeriod, ttl/2),
		taskSem:        taskSemChan,
	}

	return at
}

// Add request info to store
func (r *RequestsCounter) Add(method string) {
	r.mx.Lock()
	defer r.mx.Unlock()

	now := time.Now()
	r.requestsStore[method] = append(r.requestsStore[method], now)
}

// CoountCurrentRequests get the number of tasks in progress
func (r *RequestsCounter) CountCurrentRequests() int64 {
	return int64(len(r.taskSem))
}

// CountRecentRequests get the number of accesses in the counter time range for a specific method
func (r *RequestsCounter) CountRecentRequests() map[string]int64 {
	r.mx.Lock()
	defer r.mx.Unlock()

	result := map[string]int64{}

	for method, times := range r.requestsStore {
		result[method] = int64(len(times))
	}

	return result
}

func (r *RequestsCounter) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cleanUpPreriod)

	go func() {
		defer ticker.Stop()
	loop:
		for {
			now := time.Now()
			threshold := now.Add(-r.ttl)

			r.mx.Lock()
			for method, times := range r.requestsStore {
				var newTimes []time.Time
				for _, t := range times {
					if t.After(threshold) {
						newTimes = append(newTimes, t)
					}
				}
				r.requestsStore[method] = newTimes
			}
			r.mx.Unlock()

			select {
			case <-ticker.C:
				continue loop
			case <-ctx.Done():
				break loop
			}
		}
	}()
}
