/*
Copyright 2021 Flant JSC

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

package downtime

import (
	"context"
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"d8.io/upmeter/pkg/check"
)

type Monitor struct {
	ctx     context.Context
	cancel  context.CancelFunc
	Monitor kube_events_manager.Monitor
}

func NewMonitor(ctx context.Context) *Monitor {
	m := &Monitor{}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.Monitor = kube_events_manager.NewMonitor()
	m.Monitor.WithContext(m.ctx)
	return m
}

func (m *Monitor) Start() error {
	m.Monitor.WithConfig(&kube_events_manager.MonitorConfig{
		Metadata: struct {
			MonitorId    string
			DebugName    string
			LogLabels    map[string]string
			MetricLabels map[string]string
		}{
			"downtime-crds",
			"downtime-crds",
			map[string]string{},
			map[string]string{},
		},
		EventTypes:              nil,
		ApiVersion:              "deckhouse.io/v1alpha1",
		Kind:                    "Downtime",
		NamespaceSelector:       nil,
		LogEntry:                log.WithField("component", "downtime-monitor"),
		KeepFullObjectsInMemory: true,
	})
	// Load initial CRD list
	err := m.Monitor.CreateInformers()
	if err != nil {
		return fmt.Errorf("create informers: %v", err)
	}

	m.Monitor.Start(m.ctx)
	return m.ctx.Err()
}

func (m *Monitor) Stop() {
	m.Monitor.Stop()
}

func (m *Monitor) GetDowntimeIncidents() ([]check.DowntimeIncident, error) {
	res := make([]check.DowntimeIncident, 0)
	for _, obj := range m.Monitor.GetExistedObjects() {
		incs, err := convDowntimeIncident(obj.Object)
		if err != nil {
			return nil, err
		}
		res = append(res, incs...)
	}
	return res, nil
}

func convDowntimeIncident(obj *unstructured.Unstructured) ([]check.DowntimeIncident, error) {
	var incidentObj Downtime
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &incidentObj)
	if err != nil {
		return nil, err
	}
	return incidentObj.GetDowntimeIncidents(), nil
}
