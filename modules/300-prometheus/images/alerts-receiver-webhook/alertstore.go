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
	"k8s.io/apimachinery/pkg/api/errors"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const lastUpdatedAnnotationName = "last-updated-timestamp"

type alertItem struct {
	alert                *template.Alert
	lastReceivedTime     time.Time
	eventLastUpdatedTime time.Time
}

type alertStoreStruct struct {
	capacity int

	m      sync.RWMutex
	alerts map[string]*alertItem
}

func newStore(l int) *alertStoreStruct {
	a := make(map[string]*alertItem, l)
	return &alertStoreStruct{alerts: a, capacity: l}
}

func (a *alertStoreStruct) add(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s added to queue", alert.Fingerprint)
	a.alerts[alert.Fingerprint] = &alertItem{
		alert:            alert,
		lastReceivedTime: time.Now(),
	}
	return
}

func (a *alertStoreStruct) update(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s updated in queue", alert.Fingerprint)
	a.alerts[alert.Fingerprint].alert.Status = alert.Status
	a.alerts[alert.Fingerprint].lastReceivedTime = time.Now()
}

func (a *alertStoreStruct) remove(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s removed from queue", alert.Fingerprint)
	delete(a.alerts, alert.Fingerprint)
}

func (a *alertStoreStruct) createEvent(fingerprint string) error {
	var alert *template.Alert
	if al, ok := a.alerts[fingerprint]; ok {
		alert = al.alert
	} else {
		return fmt.Errorf("cannot find alert with fingerprint: %s", fingerprint)
	}

	log.Infof("create event with name %s", fingerprint)

	createTime := time.Now()

	msg, err := alertMessage(alert)
	if err != nil {
		return err
	}

	newEvent := &eventsv1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "events.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   nameSpace,
			Name:        fingerprint,
			Annotations: map[string]string{lastUpdatedAnnotationName: createTime.Format(time.RFC3339)},
			Labels:      map[string]string{"alert-source": "deckhouse"},
		},
		Regarding: v1.ObjectReference{
			Namespace: nameSpace,
		},
		EventTime:           metav1.NowMicro(),
		Note:                msg,
		Reason:              alert.Labels["alertname"],
		Type:                alert.Status,
		ReportingController: "prometheus",
		ReportingInstance:   "prometheus",
		Action:              alert.Status,
	}

	_, err = config.k8sClient.EventsV1().Events(nameSpace).Create(context.TODO(), newEvent, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	log.Infof("event with name %s created", fingerprint)

	a.alerts[fingerprint].eventLastUpdatedTime = createTime

	return nil
}

func (a *alertStoreStruct) updateEvent(fingerprint string) error {
	al, ok := a.alerts[fingerprint]
	if !ok {
		return fmt.Errorf("cannot find alert with fingerprint: %s", fingerprint)
	}

	// Update events one time per half-hour
	if time.Since(al.eventLastUpdatedTime) < 6*reconcileTime {
		log.Infof("event with name %s does not need updating", fingerprint)
		return nil
	}

	e, err := config.k8sClient.EventsV1().Events(nameSpace).Get(context.TODO(), fingerprint, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// event not found, create new
			return a.createEvent(fingerprint)
		}
		return err
	}

	log.Infof("update event with name %s", fingerprint)

	al.eventLastUpdatedTime = time.Now()
	if e.Annotations != nil {
		e.Annotations[lastUpdatedAnnotationName] = al.eventLastUpdatedTime.Format(time.RFC3339)
	} else {
		e.Annotations = map[string]string{lastUpdatedAnnotationName: al.eventLastUpdatedTime.Format(time.RFC3339)}
	}

	e.Type = al.alert.Status
	_, err = config.k8sClient.EventsV1().Events(nameSpace).Update(context.TODO(), e, metav1.UpdateOptions{})

	return err
}

func alertMessage(a *template.Alert) (string, error) {
	type PrintAlert struct {
		Labels      template.KV `json:"labels,omitempty" yaml:"labels,omitempty"`
		Summary     string      `json:"summary,omitempty" yaml:"summary,omitempty"`
		Description string      `json:"description,omitempty" yaml:"description,omitempty"`
		URI         string      `json:"URI,omitempty" yaml:"URI,omitempty"`
	}

	p := &PrintAlert{
		Labels:      a.Labels,
		Summary:     a.Annotations["summary"],
		Description: a.Annotations["description"],
		URI:         a.Annotations["generatorURL"],
	}

	b, err := yaml.Marshal(p)
	return string(b), err
}
