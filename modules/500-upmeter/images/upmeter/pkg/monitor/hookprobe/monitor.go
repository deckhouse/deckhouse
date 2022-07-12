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

package hookprobe

import (
	"context"
	"fmt"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Monitor struct {
	monitor kube_events_manager.Monitor
	logger  *log.Entry
}

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
			"upmeterhookprobe-monitor",
			"upmeterhookprobe-monitor",
			map[string]string{},
			map[string]string{},
		},
		EventTypes: []types.WatchEventType{
			types.WatchEventAdded,
			types.WatchEventModified,
			types.WatchEventDeleted,
		},
		ApiVersion:              "deckhouse.io/v1",
		Kind:                    "UpmeterHookProbe",
		LogEntry:                m.logger.WithField("component", "upmeterhookprobe-monitor"),
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

func (m *Monitor) getLogger() *log.Entry {
	return m.monitor.GetConfig().LogEntry
}

func (m *Monitor) Subscribe(handler Handler) {
	m.monitor.WithKubeEventCb(func(ev types.KubeEvent) {
		// One event and one object per change, we always have single item in these lists.
		evType := ev.WatchEvents[0]
		raw := ev.Objects[0].Object

		obj, err := convert(raw)
		if err != nil {
			m.getLogger().Errorf("cannot convert UpmeterHookProbe object: %v", err)
			return
		}

		switch evType {
		case types.WatchEventAdded:
			handler.OnAdd(obj)
		case types.WatchEventModified:
			handler.OnModify(obj)
		case types.WatchEventDeleted:
			handler.OnDelete(obj)
		}
	})
}

func (m *Monitor) List() ([]*HookProbe, error) {
	res := make([]*HookProbe, 0)
	for _, obj := range m.monitor.GetExistedObjects() {
		hp, err := convert(obj.Object)
		if err != nil {
			return nil, err
		}
		res = append(res, hp)
	}
	return res, nil
}

func convert(o *unstructured.Unstructured) (*HookProbe, error) {
	var hp HookProbe
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), &hp)
	if err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to v1.HookProbe: %v", err)
	}
	return &hp, nil
}

type Handler interface {
	OnAdd(*HookProbe)
	OnModify(*HookProbe)
	OnDelete(*HookProbe)
}
