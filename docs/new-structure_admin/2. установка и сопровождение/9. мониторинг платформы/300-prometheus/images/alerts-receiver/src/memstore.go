/*
Copyright 2023 Flant JSC

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

package main

import (
	"sync"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

type memStore struct {
	alerts map[model.Fingerprint]*types.Alert
	sync.RWMutex
}

func newMemStore(l int) *memStore {
	a := make(map[model.Fingerprint]*types.Alert, l)
	return &memStore{alerts: a}
}

// Add or update alert in internal store
func (a *memStore) insertAlert(alert *model.Alert) {
	a.Lock()
	defer a.Unlock()

	now := time.Now()

	removePlkAnnotations(alert)

	ta := &types.Alert{
		Alert:     *alert,
		UpdatedAt: now,
	}

	// https://github.com/prometheus/alertmanager/blob/f67d03fe2854191bb36dbcb305ec507237583aa2/api/v2/api.go#L321-L334
	// Ensure StartsAt is set.
	if ta.StartsAt.IsZero() {
		if ta.EndsAt.IsZero() {
			ta.StartsAt = now
		} else {
			ta.StartsAt = ta.EndsAt
		}
	}
	// If no end time is defined, set a timeout after which an alert
	// is marked resolved if it is not updated.
	if ta.EndsAt.IsZero() {
		ta.Timeout = true
		ta.EndsAt = now.Add(resolveTimeout)
	}
	fingerprint := ta.Fingerprint()

	if _, ok := a.alerts[fingerprint]; ok {
		log.Infof("alert with fingerprint %s updated in queue", fingerprint)
		a.alerts[fingerprint] = ta.Merge(a.alerts[fingerprint])
	} else {
		log.Infof("alert with fingerprint %s added to queue", fingerprint)
	}
	a.alerts[fingerprint] = ta

	return
}

// Remove alert from internal store
func (a *memStore) removeAlert(fingerprint model.Fingerprint) {
	a.Lock()
	defer a.Unlock()
	log.Infof("alert with fingerprint %s removed from queue", fingerprint)
	delete(a.alerts, fingerprint)
}

// Remove a bunch of alerts from internal store
func (a *memStore) removeAlerts(fingerprints []model.Fingerprint) {
	a.Lock()
	defer a.Unlock()
	for _, fingerprint := range fingerprints {
		log.Infof("alert with fingerprint %s removed from queue", fingerprint)
		delete(a.alerts, fingerprint)
	}
}

// Get alert from internal store
func (a *memStore) getAlert(fingerprint model.Fingerprint) (*types.Alert, bool) {
	a.Lock()
	defer a.Unlock()
	alert, ok := a.alerts[fingerprint]
	return alert, ok
}
