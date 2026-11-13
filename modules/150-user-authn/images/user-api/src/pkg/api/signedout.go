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
	"context"
	"slices"
	"sync"
	"time"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

const (
	// signOutWait bounds how long a successful logout waits for the cache of UserAccounts to drop
	// the devices it ended. The controller completes the UserOperation first and writes the
	// UserAccount right after, and the watch brings that write within moments.
	signOutWait = 3 * time.Second
	// cachePollInterval is how often the wait looks at the cache: a read of memory.
	cachePollInterval = 50 * time.Millisecond
	// signedOutTTL is how long a device the cache has not dropped by the end of the wait stays out
	// of the list of this replica: long enough for a slow controller, short enough to be harmless,
	// as a device ID never comes back.
	signedOutTTL = 2 * time.Minute
)

// signedOutDevices are the devices a logout of this replica ended but its cache still listed when
// the logout answered. The lists of the devices leave them out until the cache drops them or
// signedOutTTL passes.
type signedOutDevices struct {
	mu sync.Mutex
	// until is, by pair, when each device is forgotten.
	until map[string]map[string]time.Time
}

func newSignedOutDevices() *signedOutDevices {
	return &signedOutDevices{until: map[string]map[string]time.Time{}}
}

func pairOf(claims *auth.Claims) string { return claims.ConnectorID + "/" + claims.UserID }

// add remembers the devices ids of pair until the time given, and forgets the devices of every pair
// past their time.
func (s *signedOutDevices) add(pair string, ids []string, until, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for p, devices := range s.until {
		for id, t := range devices {
			if !now.Before(t) {
				delete(devices, id)
			}
		}
		if len(devices) == 0 {
			delete(s.until, p)
		}
	}
	devices := s.until[pair]
	if devices == nil {
		devices = make(map[string]time.Time, len(ids))
		s.until[pair] = devices
	}
	for _, id := range ids {
		devices[id] = until
	}
}

// filter leaves out of items the devices of pair a logout has ended, and forgets those the cache no
// longer lists or whose time has passed.
func (s *signedOutDevices) filter(pair string, items []k8s.Device, now time.Time) []k8s.Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	devices := s.until[pair]
	if len(devices) == 0 {
		return items
	}
	for id, t := range devices {
		listed := slices.ContainsFunc(items, func(d k8s.Device) bool { return d.ID == id })
		if !listed || !now.Before(t) {
			delete(devices, id)
		}
	}
	if len(devices) == 0 {
		delete(s.until, pair)
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(d k8s.Device) bool {
		_, out := devices[d.ID]
		return out
	})
}

// awaitSignedOut waits up to h.signOutWait for the cache to stop listing the devices ids of the user
// of claims, so that the list read right after a logout no longer shows them. The devices still
// listed by then stay out of the lists of this replica (signedOutDevices).
func (h *Handler) awaitSignedOut(ctx context.Context, claims *auth.Claims, ids []string) {
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, h.signOutWait)
	defer cancel()
	ticker := time.NewTicker(h.cachePollInterval)
	defer ticker.Stop()
	for {
		devices, err := h.k8sClient.Devices(ctx, claims.UserID, claims.ConnectorID)
		if err == nil {
			ids = slices.DeleteFunc(ids, func(id string) bool {
				return !slices.ContainsFunc(devices.Items, func(d k8s.Device) bool { return d.ID == id })
			})
			if len(ids) == 0 {
				return
			}
		}
		select {
		case <-ctx.Done():
			now := h.now()
			h.signedOut.add(pairOf(claims), ids, now.Add(signedOutTTL), now)
			h.logger.Info("The cache still lists devices a logout ended; they stay out of the list",
				"user_id", claims.UserID, "connector_id", claims.ConnectorID, "devices", len(ids))
			return
		case <-ticker.C:
		}
	}
}
