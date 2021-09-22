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

package crd

import (
	"context"
	"fmt"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/kube_events_manager"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "d8.io/upmeter/pkg/crd/v1"
)

type HookProbeMonitor struct {
	monitor kube_events_manager.Monitor
	logger  *log.Entry
}

func NewHookProbeMonitor(kubeClient kube.KubernetesClient, logger *log.Entry) *HookProbeMonitor {
	monitor := kube_events_manager.NewMonitor()
	monitor.WithKubeClient(kubeClient)

	return &HookProbeMonitor{
		monitor: monitor,
		logger:  logger,
	}
}

func (m *HookProbeMonitor) Start(ctx context.Context) error {
	config := &kube_events_manager.MonitorConfig{
		Metadata: struct {
			MonitorId    string
			DebugName    string
			LogLabels    map[string]string
			MetricLabels map[string]string
		}{
			"upmeterhookprobe-crd",
			"upmeterhookprobe-crd",
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

	// Load initial CRD list
	err := m.monitor.CreateInformers()
	if err != nil {
		return fmt.Errorf("create informers: %v", err)
	}

	m.monitor.Start(ctx)
	return ctx.Err()
}

func (m *HookProbeMonitor) Stop() {
	m.monitor.Stop()
}

func (m *HookProbeMonitor) getLogger() *log.Entry {
	return m.monitor.GetConfig().LogEntry
}

func (m *HookProbeMonitor) Subscribe(handler HookProbeChangeHandler) {
	m.monitor.WithKubeEventCb(func(ev types.KubeEvent) {
		// One event and one object per change, we always have single item in these lists.
		evType := ev.WatchEvents[0]
		raw := ev.Objects[0].Object

		obj, err := convertHookProbe(raw)
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

func (m *HookProbeMonitor) List() ([]*v1.HookProbe, error) {
	res := make([]*v1.HookProbe, 0)
	for _, obj := range m.monitor.GetExistedObjects() {
		rw, err := convertHookProbe(obj.Object)
		if err != nil {
			return nil, err
		}
		res = append(res, rw)
	}
	return res, nil
}

func convertHookProbe(o *unstructured.Unstructured) (*v1.HookProbe, error) {
	var rw v1.HookProbe
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), &rw)
	if err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to v1.HookProbe: %v", err)
	}
	return &rw, nil
}

type HookProbeChangeHandler interface {
	OnAdd(*v1.HookProbe)
	OnModify(*v1.HookProbe)
	OnDelete(*v1.HookProbe)
}
