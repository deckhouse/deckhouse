// Copyright 2023 Flant JSC
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

package backends

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"registry-modules-watcher/internal/metrics"
)

type Sender interface {
	Send(ctx context.Context, listBackends map[string]struct{}, versions []DocumentationTask)
}

type RegistryScanner interface {
	GetState() []DocumentationTask
	SubscribeOnUpdate(updateHandler func([]DocumentationTask) error)
}

type DocumentationTask struct {
	Registry        string
	Module          string
	Version         string
	ReleaseChannels []string
	TarFile         []byte

	Task Task
}

type Task uint

const (
	TaskCreate Task = iota
	TaskDelete
)

// BackendManager handles operations on backend endpoints and coordinates updates
type BackendManager struct {
	scanner RegistryScanner
	sender  Sender

	mu           sync.RWMutex
	backendAddrs map[string]struct{} // list of backend IP addresses
	newBackends  atomic.Uint32       // needed to calculate metrics

	logger *log.Logger
	ms     *metricstorage.MetricStorage
}

// New creates a new BackendManager instance
func New(scanner RegistryScanner, sender Sender, logger *log.Logger, ms *metricstorage.MetricStorage) *BackendManager {
	bm := &BackendManager{
		scanner:      scanner,
		sender:       sender,
		backendAddrs: make(map[string]struct{}),
		logger:       logger,
		ms:           ms,
	}

	bm.newBackends.Store(0)

	scanner.SubscribeOnUpdate(bm.handleUpdate)

	// function that will be triggered on metrics handler
	ms.AddCollectorFunc(func(s metricstorage.Storage) {
		newBackends := bm.newBackends.Load()
		s.GaugeSet(metrics.RegistryWatcherNewBackendsTotalMetric, float64(newBackends), nil)
		// Reset metric between collects
		bm.newBackends.Store(0)
	})

	return bm
}

// Add registers a new backend endpoint and sends the current documentation state
func (bm *BackendManager) Add(ctx context.Context, backend string) {
	bm.logger.Info("Adding backend", slog.String("backend", backend))

	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.backendAddrs[backend] = struct{}{}

	bm.ms.GaugeSet(metrics.RegistryWatcherBackendsTotalMetric, float64(len(bm.backendAddrs)), nil)

	state := bm.scanner.GetState()
	bm.logger.Info("Sending documentation to new backend",
		slog.String("backend", backend),
		slog.Int("docs_count", len(state)))

	bm.sender.Send(ctx, map[string]struct{}{backend: {}}, state)

	bm.newBackends.Add(1)
}

// Delete removes a backend endpoint from the managed list
func (bm *BackendManager) Delete(_ context.Context, backend string) {
	bm.logger.Info("Removing backend", slog.String("backend", backend))

	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.backendAddrs, backend)

	bm.ms.GaugeSet(metrics.RegistryWatcherBackendsTotalMetric, float64(len(bm.backendAddrs)), nil)
}

// handleUpdate sends documentation updates to all registered backends
func (bm *BackendManager) handleUpdate(docTasks []DocumentationTask) error {
	bm.logger.Info("Processing registry update event", slog.Int("tasks", len(docTasks)))

	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bm.sender.Send(context.Background(), bm.backendAddrs, docTasks)

	return nil
}
