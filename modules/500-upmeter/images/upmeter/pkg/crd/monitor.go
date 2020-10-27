package crd

import (
	"context"
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/probe/types"
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
			"monitor-crds",
			"moditor-crds",
			map[string]string{},
			map[string]string{},
		},
		EventTypes:              nil,
		ApiVersion:              "deckhouse.io/v1alpha1",
		Kind:                    "Downtime",
		NamespaceSelector:       nil,
		LogEntry:                log.WithField("component", "crd-monitor"),
		KeepFullObjectsInMemory: true,
	})
	//m.Monitor.WithKubeEventCb(func(event types.KubeEvent) {
	//	// No handler is needed.
	//})
	// Load initial CRD list
	err := m.Monitor.CreateInformers()
	if err != nil {
		return fmt.Errorf("create informers: %v", err)
	}

	m.Monitor.Start(m.ctx)
	return nil
}

func (m *Monitor) Stop() {
	m.Monitor.Stop()
}

func (m *Monitor) GetDowntimeIncidents() []types.DowntimeIncident {
	res := make([]types.DowntimeIncident, 0)
	for _, obj := range m.Monitor.GetExistedObjects() {
		res = append(res, ConvertToDowntimeIncidents(obj.Object)...)
	}
	return res
}

func (m *Monitor) FilterDowntimeIncidents(from, to int64, group string, muteDowntimeTypes []string) []types.DowntimeIncident {
	res := make([]types.DowntimeIncident, 0)
	for _, obj := range m.Monitor.GetExistedObjects() {
		incidents := ConvertToDowntimeIncidents(obj.Object)
		for _, incident := range incidents {
			// filter out by time
			if incident.End <= from || incident.Start >= to {
				continue
			}
			// filter by group name
			var hasGroup = false
			for _, groupName := range incident.Affected {
				if group == groupName {
					hasGroup = true
				}
			}
			if !hasGroup {
				continue
			}
			// filter non-interesting types
			isMuted := false
			for _, mutedType := range muteDowntimeTypes {
				if mutedType == incident.Type {
					isMuted = true
				}
			}
			if !isMuted {
				continue
			}

			res = append(res, incident)
		}
	}
	return res
}
