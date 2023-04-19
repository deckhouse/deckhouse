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
	"time"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type AlertItem struct {
	Alert            *template.Alert
	LastReceivedTime time.Time
}

type EventItem struct {
	Name           string
	LastUpdateTime time.Time
}

type AlertStore struct {
	m      sync.RWMutex
	length int
	Alerts map[string]*AlertItem
	Events map[string]*EventItem
}

func NewStore(l int) *AlertStore {
	a := make(map[string]*AlertItem, l)
	e := make(map[string]*EventItem, l)
	return &AlertStore{Alerts: a, Events: e, length: l}
}

func (a *AlertStore) Add(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s added to queue", alert.Fingerprint)
	a.Alerts[alert.Fingerprint] = &AlertItem{
		Alert:            alert,
		LastReceivedTime: time.Now(),
	}
	return
}

func (a *AlertStore) Update(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s updated in queue", alert.Fingerprint)
	a.Alerts[alert.Fingerprint].LastReceivedTime = time.Now()
}

func (a *AlertStore) Remove(alert *template.Alert) {
	a.m.Lock()
	defer a.m.Unlock()
	log.Infof("alert with fingerprint %s removed from queue", alert.Fingerprint)
	delete(a.Alerts, alert.Fingerprint)
}

func (a *AlertStore) CreateEvent(fingerprint string) error {
	var alert *template.Alert
	if al, ok := a.Alerts[fingerprint]; ok {
		alert = al.Alert
	} else {
		return fmt.Errorf("cannot find alert with fingerprint: %s", fingerprint)
	}

	log.Infof("create event with fingerprint %s", fingerprint)

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
			Namespace:    nameSpace,
			GenerateName: "prometheus-alert-",
			Annotations: map[string]string{"lastUpdated": createTime.Format(time.RFC3339) },
		},
		Regarding: v1.ObjectReference{
			Namespace: nameSpace,
		},
		EventTime:           metav1.NowMicro(),
		Note:                msg,
		Reason:              alert.Labels["alertname"],
		Type:                v1.EventTypeWarning,
		ReportingController: "prometheus",
		ReportingInstance:   "prometheus",
		Action:              alert.Status,
	}
	e, err := config.K8sClient.EventsV1().Events(nameSpace).Create(context.TODO(), newEvent, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	log.Infof("event with fingerprint %s and name %s created", fingerprint, e.Name)
	a.Events[fingerprint] = &EventItem{
		Name:           e.Name,
		LastUpdateTime: createTime,
	}
	return nil
}

func (a *AlertStore) UpdateEvent(fingerprint string) error {
	ev, ok := a.Events[fingerprint]
	if !ok {
		return fmt.Errorf("cannot find event with fingerprint: %s", fingerprint)
	}

	// Update events one time per half-hour
	if time.Since(ev.LastUpdateTime) < 30*time.Minute {
		log.Infof("event with fingerprint %s and name %s does not need updating", fingerprint, ev.Name)
		return nil
	}

	log.Infof("update event with fingerprint %s and name %s", fingerprint, ev.Name)

	e, err := config.K8sClient.EventsV1().Events(nameSpace).Get(context.TODO(), ev.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ev.LastUpdateTime = time.Now()
	if e.Annotations != nil {
		e.Annotations["lastUpdated"] = ev.LastUpdateTime.Format(time.RFC3339)
	} else {
		e.Annotations = map[string]string{"lastUpdated": ev.LastUpdateTime.Format(time.RFC3339)}
	}
	_, err = config.K8sClient.EventsV1().Events(nameSpace).Update(context.TODO(), e, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (a *AlertStore) RemoveEvent(fingerprint string) error {
	ev, ok := a.Events[fingerprint]
	if !ok {
		return fmt.Errorf("cannot find event with fingerprint: %s", fingerprint)
	}

	log.Infof("remove event with fingerprint %s and name %s", fingerprint, ev.Name)

	return config.K8sClient.EventsV1().Events(nameSpace).Delete(context.TODO(), ev.Name, metav1.DeleteOptions{})
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
