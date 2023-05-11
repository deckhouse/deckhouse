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

type alertStoreStruct struct {
	capacity int

	sync.RWMutex
	alerts map[model.Fingerprint]*types.Alert
}

func newStore(l int) *alertStoreStruct {
	a := make(map[model.Fingerprint]*types.Alert, l)
	return &alertStoreStruct{alerts: a, capacity: l}
}

func (a *alertStoreStruct) add(alert *model.Alert) {
	a.Lock()
	defer a.Unlock()

	fingerprint := alert.Fingerprint()
	log.Infof("alert with fingerprint %s added to queue", fingerprint)
	a.alerts[fingerprint] = &types.Alert{
		Alert: *alert,
		UpdatedAt: time.Now(),
	}

	return
}

func (a *alertStoreStruct) update(alert *model.Alert) {
	a.Lock()
	defer a.Unlock()
	fingerprint := alert.Fingerprint()
	log.Infof("alert with fingerprint %s updated in queue", fingerprint)
	a.alerts[fingerprint].UpdatedAt = time.Now()
}

func (a *alertStoreStruct) remove(alert *model.Alert) {
	a.Lock()
	defer a.Unlock()
	fingerprint := alert.Fingerprint()
	log.Infof("alert with fingerprint %s removed from queue", fingerprint)
	delete(a.alerts, fingerprint)
}
