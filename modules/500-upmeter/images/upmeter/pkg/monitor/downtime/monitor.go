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

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"d8.io/upmeter/pkg/check"
)

type Monitor struct {
	monitor kube_events_manager.Monitor
	logger  *log.Entry
}

/*func NewMonitor(ctx context.Context) *Monitor {
	m := &Monitor{}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.monitor = kube_events_manager.NewMonitor()
	m.monitor.WithContext(m.ctx)
	return m
}*/

func NewMonitor(kubeClient kube.KubernetesClient, logger *log.Entry) *Monitor {
	monitor := kube_events_manager.NewMonitor()
	monitor.WithKubeClient(kubeClient)

	return &Monitor{
		monitor: monitor,
		logger:  logger,
	}
}

func (m *Monitor) Start(ctx context.Context) error {
	config := &kube_events_manager.MonitorConfig{
		Metadata: struct {
			MonitorId    string
			DebugName    string
			LogLabels    map[string]string
			MetricLabels map[string]string
		}{
			"downtime-monitor",
			"downtime-monitor",
			map[string]string{},
			map[string]string{},
		},
		EventTypes:              nil,
		ApiVersion:              "deckhouse.io/v1alpha1",
		Kind:                    "Downtime",
		NamespaceSelector:       nil,
		LogEntry:                log.WithField("component", "downtime-monitor"),
		KeepFullObjectsInMemory: true,
	}

	m.monitor.WithContext(ctx)
	m.monitor.WithConfig(config)

	err := m.monitor.CreateInformers()
	if err != nil {
		return fmt.Errorf("creating informer: %v", err)
	}

	m.monitor.Start(ctx)
	return nil
}

func (m *Monitor) Stop() {
	m.monitor.Stop()
}

func (m *Monitor) List() ([]check.DowntimeIncident, error) {
	res := make([]check.DowntimeIncident, 0)
	for _, obj := range m.monitor.GetExistedObjects() {
		incs, err := convert(obj.Object)
		if err != nil {
			return nil, err
		}
		res = append(res, incs...)
	}
	return res, nil
}

func convert(obj *unstructured.Unstructured) ([]check.DowntimeIncident, error) {
	var incidentObj Downtime
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &incidentObj)
	if err != nil {
		return nil, err
	}
	return incidentObj.GetDowntimeIncidents(), nil
}
