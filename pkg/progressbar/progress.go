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

package progressbar

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/term"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
)

// formatBytes selects an adaptive unit of measurement and formats the value
// Returns a formatted string with the selected unit of measurement
func formatBytes(bytes int64, precision int) (float64, string) {
	var divisor float64
	var unit string

	switch {
	case bytes >= GB:
		divisor = float64(GB)
		unit = "GiB"
	case bytes >= MB:
		divisor = float64(MB)
		unit = "MiB"
	case bytes >= KB:
		divisor = float64(KB)
		unit = "KiB"
	default:
		divisor = 1
		unit = "B"
	}

	return float64(bytes) / divisor, unit
}

// adaptiveFormatDecorator returns relative progress display with adaptive unit of measurement
// Selects unit (B, KiB, MiB, GiB) depending on file size (by total)
func adaptiveFormatDecorator(wc decor.WC) decor.Decorator {
	decoratorFn := decor.DecorFunc(func(stats decor.Statistics) string {
		// Use unit determined by total size for both values
		totalVal, unit := formatBytes(stats.Total, 2)

		var divisor float64
		switch unit {
		case "GiB":
			divisor = float64(GB)
		case "MiB":
			divisor = float64(MB)
		case "KiB":
			divisor = float64(KB)
		default:
			divisor = 1
		}

		currentVal := float64(stats.Current) / divisor

		// Round to whole number for bytes
		if unit == "B" {
			return fmt.Sprintf("%.0f%s/%.0f%s", currentVal, unit, totalVal, unit)
		}

		return fmt.Sprintf("%.2f%s/%.2f%s", currentVal, unit, totalVal, unit)
	})
	return decor.Any(decoratorFn, wc)
}

// downloadedBytesDecorator returns a custom decorator for displaying the amount of downloaded data
// Used for "Waiting" mode when size is unknown in advance (total < 0)
// Takes a Task pointer to access atomicDownloadedBytes
func downloadedBytesDecorator(task *Task, wc decor.WC) decor.Decorator {
	fn := decor.DecorFunc(func(s decor.Statistics) string {
		// Get the actual number of downloaded bytes from atomic variable
		downloadedBytes := task.downloadedBytes.Load()

		currentVal, unit := formatBytes(downloadedBytes, 2)

		// Round to whole number for bytes
		if unit == "B" {
			return fmt.Sprintf("%.0f%s", currentVal, unit)
		}

		return fmt.Sprintf("%.2f%s", currentVal, unit)
	})
	return decor.Any(fn, wc)
}

// statusDecorator returns a dynamic status decorator for tasks with unknown size
// Shows "Waiting" while nothing is downloaded and "Downloading" when download starts
func statusDecorator(wc decor.WC) decor.Decorator {
	fn := decor.DecorFunc(func(s decor.Statistics) string {
		if s.Completed {
			return "Downloaded"
		}
		if s.Current > 0 {
			return "Downloading"
		}
		return "Waiting"
	})
	return decor.Any(fn, wc)
}

// Manager manages the display of progress bars for multi-threaded dependency loading.
// It wraps the mpb container and provides thread-safe operations similar to Docker's output.
type Manager struct {
	enabled bool
	mu      sync.Mutex
	tasks   map[string]*Task
	p       *mpb.Progress
}

// Task represents a single progress bar task, wrapping an mpb bar.
type Task struct {
	id              string
	bar             *mpb.Bar
	total           int64
	downloadedBytes atomic.Int64 // Tracks actual downloaded bytes for unknown-size downloads
}

type TaskType int

const (
	TaskTypeBytes   TaskType = iota // For downloads (KiB, MiB)
	TaskTypeNumeric                 // For steps (1 / 10)
	TaskTypeSpinner                 // For indeterminate tasks
)

// NewManager creates a new progress manager.
// It automatically detects TTY and CI environment to decide whether to display progress bars.
// If TTY is not available or CI=true, it operates in no-op mode (no bars displayed).
func NewManager() *Manager {
	enabled := isTTYAvailable()

	var p *mpb.Progress
	if enabled {
		p = mpb.New(mpb.WithWidth(1))
	}

	return &Manager{
		enabled: enabled,
		tasks:   make(map[string]*Task),
		p:       p,
	}
}

func (m *Manager) AddTask(id string, total, current int64, taskType TaskType) *Task {
	if total < 0 {
		total = -1
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[id]; exists {
		return task
	}

	task := &Task{id: id, total: total}
	if !m.enabled {
		m.tasks[id] = task
		return task
	}

	var barOptions []mpb.BarOption

	// 1. Name (ID)
	barOptions = append(barOptions, mpb.PrependDecorators(
		decor.Name(id+":", decor.WC{W: 15, C: decor.DindentRight}),
	))

	if total < 0 {
		barOptions = append(barOptions, mpb.AppendDecorators(
			statusDecorator(decor.WC{W: 12}),
			downloadedBytesDecorator(task, decor.WC{W: 28}),
		))
		// Use maximum int64 value for Waiting mode so the bar never fills up,
		// but the decorator can show current progress
		task.bar = m.p.AddBar(1<<63-1, barOptions...)
	} else {
		switch taskType {
		case TaskTypeBytes:
			barOptions = append(barOptions, mpb.AppendDecorators(
				// Status: changes to "Complete" upon completion
				decor.OnComplete(
					decor.Name("Downloading", decor.WC{W: 13}),
					"Downloaded",
				),
				// ETA: shown only while downloading
				decor.OnComplete(
					decor.AverageETA(decor.ET_STYLE_GO, decor.WC{W: 8}),
					"",
				),
				// decor.Name(" ", decor.WC{W: 1}),
				// Our MB counters
				adaptiveFormatDecorator(decor.WC{W: 20}),
			))

		case TaskTypeNumeric:
			barOptions = append(barOptions, mpb.AppendDecorators(
				decor.Percentage(decor.WC{W: 5}),
				decor.CountersNoUnit(" (%d/%d)", decor.WC{W: 10}),
			))
		}

		task.bar = m.p.AddBar(total, barOptions...)
		if current > 0 {
			task.bar.SetCurrent(current)
		}
	}

	m.tasks[id] = task
	return task
}

// Increment advances a task's progress by n bytes.
func (m *Manager) Increment(id string, n int64) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("task %q not found", id)
	}

	if !m.enabled || task.bar == nil {
		return nil
	}

	// Update atomic counter for unknown-size downloads (total < 0)
	if task.total < 0 {
		task.downloadedBytes.Add(n)
	}

	task.bar.IncrInt64(n)
	return nil
}

// SetCurrent sets the absolute progress for a task.
func (m *Manager) SetCurrent(id string, current int64) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("task %q not found", id)
	}

	if !m.enabled || task.bar == nil {
		return nil
	}

	// Update atomic counter for unknown-size downloads (total < 0)
	if task.total < 0 {
		task.downloadedBytes.Store(current)
	}

	task.bar.SetCurrent(current)
	return nil
}

// Complete marks a task as completed.
func (m *Manager) Complete(id string) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("task %q not found", id)
	}

	if !m.enabled || task.bar == nil {
		return nil
	}

	if task.total < 0 {
		// For spinners, set max value to trigger Completed in statusDecorator
		task.bar.SetCurrent(1<<63 - 1)
	} else {
		task.bar.SetCurrent(task.total)
	}
	return nil
}

// WaitAndClose waits for all bars to complete and closes the manager.
// This should be called when all downloads are finished.
func (m *Manager) WaitAndClose() error {
	if !m.enabled || m.p == nil {
		return nil
	}

	m.p.Wait()
	return nil
}

// WrapReader wraps an io.Reader with progress tracking.
// Each byte read from the reader increments the progress bar.
func (t *Task) WrapReader(r io.Reader) io.Reader {
	if t.bar == nil {
		// In no-op mode, return the original reader
		return r
	}

	return t.bar.ProxyReader(r)
}

// isTTYAvailable checks if the output is a TTY and if CI environment variable is not set.
// Returns false if either condition is not met (ensuring no-op mode for CI/CD environments).
// Uses stderr for TTY detection (standard for progress bars in CLI tools).
func isTTYAvailable() bool {
	// Check if CI environment variable is set
	if _, ok := os.LookupEnv("CI"); ok {
		return false
	}

	// Check if stderr is a terminal using golang.org/x/term (standard de-facto)
	return term.IsTerminal(int(os.Stderr.Fd()))
}
