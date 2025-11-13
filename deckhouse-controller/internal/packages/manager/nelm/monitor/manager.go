// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package monitor provides resource monitoring for Helm releases deployed via nelm.
// It periodically checks that all resources from a release manifest are present in the cluster,
// helping detect configuration drift or accidental deletions.
package monitor

import (
	"context"
	"sync"

	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Manager coordinates multiple resource monitors for Helm releases.
// Thread-safe for concurrent access.
type Manager struct {
	ctx context.Context

	cache runtimecache.Cache
	nelm  *nelm.Client

	mtx      sync.Mutex                   // protects monitors map
	monitors map[string]*resourcesMonitor // keyed by Helm release name

	callback AbsentCallback

	logger *log.Logger
}

// New creates a new monitor manager instance.
func New(cache runtimecache.Cache, nelm *nelm.Client, callback AbsentCallback, logger *log.Logger) *Manager {
	return &Manager{
		ctx:      context.Background(),
		cache:    cache,
		nelm:     nelm,
		callback: callback,
		monitors: make(map[string]*resourcesMonitor),
		logger:   logger,
	}
}

// CheckResources performs an immediate check of resources for a specific release.
// Returns nil if monitor doesn't exist or all resources are present.
func (m *Manager) CheckResources(ctx context.Context, name string) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.monitors[name]; !ok {
		return nil
	}

	return m.monitors[name].checkResources(ctx)
}

// AddMonitor creates and starts a new monitor for a Helm release.
// If a monitor already exists for this release, stop it and start a new one.
// The monitor will run in the background, checking resources every 4 minutes.
func (m *Manager) AddMonitor(name, rendered string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.monitors[name]; ok {
		m.monitors[name].Stop()
	}

	m.monitors[name] = newMonitor(m.cache, m.nelm, name, rendered, m.logger)
	m.monitors[name].Start(m.ctx, m.callback)
}

// RemoveMonitor stops and removes a monitor for a Helm release.
// If the monitor doesn't exist, the call is a no-op.
func (m *Manager) RemoveMonitor(name string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.monitors[name]; !ok {
		return
	}

	m.monitors[name].Stop()
	delete(m.monitors, name)
}

// HasMonitor checks if the monitor exists
func (m *Manager) HasMonitor(name string) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, ok := m.monitors[name]
	return ok
}

// PauseMonitor increments the pause counter for a release monitor.
// The monitor will skip resource checks while paused.
// Safe to call from multiple goroutines; requires equal Resume calls to unpause.
func (m *Manager) PauseMonitor(name string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.monitors[name]; !ok {
		return
	}

	m.monitors[name].Pause()
}

// ResumeMonitor decrements the pause counter for a release monitor.
// The monitor resumes checking resources when counter reaches zero.
func (m *Manager) ResumeMonitor(name string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.monitors[name]; !ok {
		return
	}

	m.monitors[name].Resume()
}

// Stop gracefully shuts down all monitors.
// Blocks until all monitor goroutines have exited.
func (m *Manager) Stop() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.logger.Debug("stop monitors")

	for name, monitor := range m.monitors {
		monitor.Stop()
		delete(m.monitors, name)
	}
}
