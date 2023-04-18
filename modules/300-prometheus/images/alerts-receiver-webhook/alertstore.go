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
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AlertStore struct {
	m      sync.RWMutex
	length int
	Alerts map[string]*template.Alert
	Events map[string]*eventsv1.Event
}

func NewStore(l int) *AlertStore {
	a := make(map[string]*template.Alert, l)
	e := make(map[string]*eventsv1.Event, l)
	return &AlertStore{Alerts: a, Events: e, length: l}
}

func (a *AlertStore) Add(alert template.Alert) error {
	if len(a.Alerts) == a.length {
		return fmt.Errorf("cannot add alert to queue (max length = %d), queue is full", a.length)
	}

	if alert.Labels["alertname"] == "DeadMansSwitch" {
		log.Debug("skip DeadMansSwitch alert")
		return nil
	}

	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s added to queue", alert.Fingerprint)
	a.Alerts[alert.Fingerprint] = &alert
	return nil
}

func (a *AlertStore) CreateEvent(fingerprint string) error {
	alert, ok := a.Alerts[fingerprint]
	if !ok {
		return fmt.Errorf("cannot find alert with fingerprint: %s", fingerprint)
	}

	ev := &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nameSpace,
			Name:      fingerprint,
		},
		Regarding: v1.ObjectReference{
			Namespace: nameSpace,
		},
		EventTime:           metav1.NowMicro(),
		Note:                alertMessage(alert),
		Reason:              alert.Labels["alertname"],
		Type:                v1.EventTypeWarning,
		ReportingController: "prometheus",
		ReportingInstance:   "prometheus",
		Action:              alert.Status,
	}
	e, err := config.K8sClient.EventsV1().Events(nameSpace).Create(context.TODO(), ev, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	a.Events[fingerprint] = e
	return nil
}

func alertMessage(a *template.Alert) string {
	const format = `Labels:
%s
Summary: %s
Description: %s
Url: %s
`
	var labels string

	for k, v := range a.Labels {
		labels = fmt.Sprintf("\t%s: %s\n", k, v)
	}

	return fmt.Sprintf(format, labels, a.Annotations["summary"], a.Annotations["description"], a.Annotations["generatorURL"])
}
