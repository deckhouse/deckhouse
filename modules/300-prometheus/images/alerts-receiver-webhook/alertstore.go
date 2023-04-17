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
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AlertItem struct {
	Alert template.Alert
}

type AlertStore struct {
	m      sync.RWMutex
	length int
	Alerts map[string]*AlertItem
	Events map[string]*eventsv1.Event
}

func NewStore(l int) *AlertStore {
	a := make(map[string]*AlertItem, l)
	e := make(map[string]*eventsv1.Event, l)
	return &AlertStore{Alerts: a, Events: e, length: l}
}

func (a *AlertStore) Add(alert template.Alert) error {
	if len(a.Alerts) == a.length {
		return fmt.Errorf("cannot add alert to queue (max length = %d), queue is full", a.length)
	}
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s added to queue", alert.Fingerprint)
	a.Alerts[alert.Fingerprint] = &AlertItem{
		Alert: alert,
	}
	return nil
}

func (a *AlertStore) CreateEvent(fingerprint string) error {
	ev := &eventsv1.Event{}
	e, err := config.K8sClient.EventsV1().Events(NameSpace).Create(context.TODO(), ev, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	a.Events[fingerprint] = e
	return nil
}
