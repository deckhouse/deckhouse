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

package cron

import (
	"context"
	"log/slog"
	"sync"

	schedulemanager "github.com/flant/shell-operator/pkg/schedule_manager"
	shtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
	"gopkg.in/robfig/cron.v2"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ schedulemanager.ScheduleManager = &Manager{}

// Manager coordinates cron-based scheduling, allowing multiple schedule entries
// to share the same crontab expression while maintaining independent IDs.
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	once sync.Once
	wg   sync.WaitGroup // Tracks lifecycle goroutines

	cron       *cron.Cron
	scheduleCh chan string // Receives crontab expressions when schedules fire

	mu      sync.Mutex       // Protects entries map
	entries map[string]entry // Maps crontab expression to entry details

	logger *log.Logger
}

// entry tracks a single cron job that may be shared by multiple schedule IDs.
// This allows deduplication when multiple schedules use the same crontab expression.
type entry struct {
	entryID cron.EntryID        // Reference to the underlying cron job
	ids     map[string]struct{} // Set of schedule IDs sharing this crontab
}

// NewManager creates a new cron Manager with the given context and logger.
func NewManager(logger *log.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:        ctx,
		cancel:     cancel,
		scheduleCh: make(chan string, 1), // Buffered to prevent blocking a single slow consumer

		cron:    cron.New(),
		entries: make(map[string]entry),

		logger: logger.Named("cron-manager"),
	}
}

// Start begins the cron scheduler. Can be called multiple times safely (only runs once).
func (m *Manager) Start() {
	m.once.Do(func() {
		m.logger.Info("start cron")

		m.cron.Start()

		// Launch goroutine to gracefully stop cron when context is cancelled
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()

			<-m.ctx.Done()
			m.cron.Stop()
		}()
	})
}

// Stop cancels the context and waits for the lifecycle goroutines to finish.
func (m *Manager) Stop() {
	m.logger.Debug("stop cron")

	m.cancel()
	m.wg.Wait()
}

// Ch returns the channel that receives crontab expressions when schedules fire.
func (m *Manager) Ch() chan string {
	return m.scheduleCh
}

// Add registers a schedule entry. If the crontab expression is already registered,
// the schedule ID is added to the existing entry. Otherwise, a new cron job is created.
// Invalid crontab expressions are logged and ignored.
func (m *Manager) Add(schedule shtypes.ScheduleEntry) {
	if schedule.Crontab == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("add cron entry", slog.String("cron", schedule.Crontab))

	// Check if this crontab expression already exists
	if e, has := m.entries[schedule.Crontab]; has {
		// Add this schedule ID to the existing entry if not already present
		if _, has = e.ids[schedule.Id]; !has {
			m.entries[schedule.Crontab].ids[schedule.Id] = struct{}{}
		}

		return
	}

	// Create a new cron job for this crontab expression
	entryID, err := m.cron.AddFunc(schedule.Crontab, func() {
		m.logger.Debug("cron job scheduled", slog.String("cron", schedule.Crontab))
		m.scheduleCh <- schedule.Crontab
	})
	if err != nil {
		// Log invalid crontab expressions instead of silently failing
		m.logger.Error("failed to add cron schedule", "crontab", schedule.Crontab, "id", schedule.Id, "error", err)
		return
	}

	// Register the new entry
	m.entries[schedule.Crontab] = entry{
		entryID: entryID,
		ids: map[string]struct{}{
			schedule.Id: {},
		},
	}
}

// Remove unregisters a schedule entry. If this is the last schedule ID using
// the crontab expression, the underlying cron job is stopped and removed.
func (m *Manager) Remove(schedule shtypes.ScheduleEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("remove cron entry", slog.String("cron", schedule.Crontab))

	e, has := m.entries[schedule.Crontab]
	if !has {
		return
	}

	if _, has = e.ids[schedule.Id]; !has {
		return
	}

	// Remove this schedule ID from the entry
	delete(m.entries[schedule.Crontab].ids, schedule.Id)

	// If no more schedule IDs reference this crontab, remove the cron job
	if len(m.entries[schedule.Crontab].ids) == 0 {
		m.cron.Remove(m.entries[schedule.Crontab].entryID)
		delete(m.entries, schedule.Crontab)
	}
}
