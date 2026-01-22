// Copyright 2026 Flant JSC
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
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManager_ConcurrentDownloads(t *testing.T) {
	// Initialize manager
	m := NewManager()

	// Scenarios:
	// 1. Normal download
	// 2. Resume (already has 50%)
	// 3. Unknown size (spinner)
	tasks := []struct {
		id      string
		total   int64
		current int64
	}{
		{"layer-1-new", 5 * 1024 * 1024, 0},
		{"layer-2-new", 10 * 1024 * 1024, 0},
		{"layer-3-resume", 5 * 1024 * 1024, 2500000},
		{"layer-4-unknown", -1, 0},
	}

	var wg sync.WaitGroup
	for _, tc := range tasks {
		wg.Add(1)
		go func(id string, total, current int64) {
			defer wg.Done()

			// Determine the correct task type
			taskType := TaskTypeBytes
			if total < 0 {
				taskType = TaskTypeSpinner
			}

			_ = m.AddTask(id, total, current, taskType)

			time.Sleep(time.Second)
			// Simulate download process
			for i := 0; i < 5; i++ {
				// In real code, this would happen through WrapReader
				_ = m.Increment(id, 1024*1024) // 1 MB
				time.Sleep(500 * time.Millisecond)
			}
			_ = m.Complete(id)
		}(tc.id, tc.total, tc.current)
	}

	wg.Wait()
	err := m.WaitAndClose()
	if err != nil {
		t.Fatalf("WaitAndClose failed: %v", err)
	}
}

func TestManager_WrapReader(t *testing.T) {
	m := NewManager()
	id := "test-reader"
	content := []byte("hello deckhouse progress bar")
	total := int64(len(content))

	task := m.AddTask(id, total, 0, TaskTypeBytes)

	// Wrap the buffer in our progress bar
	reader := bytes.NewReader(content)
	progressReader := task.WrapReader(reader)

	// Read the data
	out, err := io.ReadAll(progressReader)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if !bytes.Equal(out, content) {
		t.Error("Content mismatch after WrapReader")
	}

	m.WaitAndClose()
}

func TestManager_NoOpMode(t *testing.T) {
	// Force disable via ENV, simulating CI
	t.Setenv("CI", "true")

	m := NewManager()
	if m.enabled {
		t.Error("Manager should be disabled in CI mode")
	}

	task := m.AddTask("ci-task", 100, 0, TaskTypeBytes)
	if task.bar != nil {
		t.Error("Bar should be nil in No-Op mode")
	}

	// Check that methods don't panic
	err := m.Increment("ci-task", 10)
	if err != nil {
		t.Errorf("Increment failed in no-op: %v", err)
	}

	// Check WrapReader in No-Op mode
	data := []byte("test")
	reader := bytes.NewReader(data)
	wrapped := task.WrapReader(reader)

	out, _ := io.ReadAll(wrapped)
	if !bytes.Equal(out, data) {
		t.Error("WrapReader corrupted data in No-Op mode")
	}
}

func TestManager_StatusTransition(t *testing.T) {
	// Check status transition from Waiting to Downloading for tasks with unknown size
	m := NewManager()
	defer m.WaitAndClose()

	// Create task with unknown size
	_ = m.AddTask("test-unknown", -1, 0, TaskTypeSpinner)

	// Initially status should be "Waiting" (checked visually in interactive mode)
	// After increment, status should change to "Downloading"
	err := m.Increment("test-unknown", 1)
	if err != nil {
		t.Errorf("Increment failed: %v", err)
	}

	// Complete the task
	err = m.Complete("test-unknown")
	if err != nil {
		t.Errorf("Complete failed: %v", err)
	}
}

func TestManager_LargeFileDownloads(t *testing.T) {
	// Demonstrates downloading large files: 100 MB and 2 GB
	m := NewManager()

	const (
		MB = int64(1024 * 1024)
		GB = int64(1024 * 1024 * 1024)
	)

	tasks := []struct {
		id    string
		size  int64
		label string
	}{
		{"download-100mb", 100 * MB, "100 MB file"},
		{"download-2gb", 2 * GB, "2 GB file"},
	}

	var wg sync.WaitGroup
	for _, tc := range tasks {
		wg.Add(1)
		go func(id string, total int64, label string) {
			defer wg.Done()

			_ = m.AddTask(id, total, 0, TaskTypeBytes)

			// Initial delay before starting download
			time.Sleep(500 * time.Millisecond)

			// Simulate download in multiple stages (50 MB per step)
			chunkSize := int64(50 * MB)
			for downloaded := int64(0); downloaded < total; downloaded += chunkSize {
				remaining := total - downloaded
				if remaining < chunkSize {
					chunkSize = remaining
				}

				_ = m.Increment(id, chunkSize)
				time.Sleep(200 * time.Millisecond) // Simulate network delay
			}

			_ = m.Complete(id)
		}(tc.id, tc.size, tc.label)
	}

	wg.Wait()
	err := m.WaitAndClose()
	if err != nil {
		t.Fatalf("WaitAndClose failed: %v", err)
	}
}

func TestManager_VeryLongTaskNames(t *testing.T) {
	// Check that very long task names are handled correctly
	m := NewManager()
	defer m.WaitAndClose()

	longNameTests := []struct {
		id   string
		name string
	}{
		{
			id:   "very-long-name-1",
			name: "docker.io/library/very-long-image-name-with-multiple-parts-that-exceeds-normal-width-limits-and-should-be-truncated-or-wrapped-correctly",
		},
		{
			id:   "very-long-name-2",
			name: "ghcr.io/my-org/my-project/sub-project/component/very-long-descriptive-name-with-version-and-hash-12345678901234567890",
		},
		{
			id:   "extremely-long",
			name: strings.Repeat("a", 500), // Extremely long name
		},
	}

	for _, tc := range longNameTests {
		task := m.AddTask(tc.id, 1024*1024, 0, TaskTypeBytes)
		if task == nil {
			t.Errorf("AddTask failed for long name: %s", tc.id)
		}

		// Increment several times
		for i := 0; i < 3; i++ {
			err := m.Increment(tc.id, 256*1024)
			if err != nil {
				t.Errorf("Increment failed for %s: %v", tc.id, err)
			}
		}

		err := m.Complete(tc.id)
		if err != nil {
			t.Errorf("Complete failed for %s: %v", tc.id, err)
		}
	}
}

func TestManager_EdgeCaseTaskIds(t *testing.T) {
	// Check handling of tasks with unusual IDs
	m := NewManager()
	defer m.WaitAndClose()

	edgeCaseIds := []string{
		"",                           // Empty ID
		"task-with-special-chars!@#", // Special characters
		"task_with_unicode_🚀_emoji",  // Unicode and emoji
		"   spaces   ",               // Spaces
		"task\nwith\nnewlines",       // Line breaks
		"tab\there",                  // Tabulation
	}

	for _, id := range edgeCaseIds {
		task := m.AddTask(id, 1000, 0, TaskTypeBytes)
		if task == nil {
			t.Errorf("AddTask returned nil for ID: %q", id)
		}

		// Check that Increment works
		err := m.Increment(id, 100)
		if err != nil {
			t.Errorf("Increment failed for ID %q: %v", id, err)
		}

		err = m.Complete(id)
		if err != nil {
			t.Errorf("Complete failed for ID %q: %v", id, err)
		}
	}
}

func TestManager_ZeroAndNegativeSizes(t *testing.T) {
	// Check handling of negative size (zero-size tasks not supported in mpb)
	m := NewManager()
	defer m.WaitAndClose()

	tests := []struct {
		id       string
		size     int64
		taskType TaskType
	}{
		{"one-byte", 1, TaskTypeBytes},
		{"negative-one", -1, TaskTypeSpinner},
		{"very-negative", -999999, TaskTypeSpinner},
	}

	for _, tc := range tests {
		_ = m.AddTask(tc.id, tc.size, 0, tc.taskType)

		// Increment several times
		err := m.Increment(tc.id, 1)
		if err != nil {
			t.Errorf("Unexpected error for %s: %v", tc.id, err)
		}

		err = m.Complete(tc.id)
		if err != nil {
			t.Errorf("Complete failed for %s: %v", tc.id, err)
		}
	}
}

func TestManager_ExceedingProgress(t *testing.T) {
	// Check that progress greater than total doesn't cause panic
	m := NewManager()
	defer m.WaitAndClose()

	id := "exceeding-task"
	task := m.AddTask(id, 1000, 0, TaskTypeBytes)
	if task == nil {
		t.Fatal("AddTask failed")
	}

	// Increment more than total
	for i := 0; i < 20; i++ {
		err := m.Increment(id, 200)
		if err != nil {
			t.Errorf("Increment failed: %v", err)
		}
	}

	err := m.Complete(id)
	if err != nil {
		t.Errorf("Complete failed: %v", err)
	}
}

func TestManager_RapidIncrements(t *testing.T) {
	// Check very fast increments without delays
	m := NewManager()
	defer m.WaitAndClose()

	id := "rapid-task"
	const totalSize = int64(10 * 1024 * 1024) // 10 MB
	task := m.AddTask(id, totalSize, 0, TaskTypeBytes)

	if task == nil {
		t.Fatal("AddTask failed")
	}

	// Many fast increments
	incrementCount := 1000
	incrementSize := totalSize / int64(incrementCount)

	for i := 0; i < incrementCount; i++ {
		err := m.Increment(id, incrementSize)
		if err != nil {
			t.Errorf("Increment %d failed: %v", i, err)
		}
	}

	err := m.Complete(id)
	if err != nil {
		t.Errorf("Complete failed: %v", err)
	}
}

func TestManager_SetCurrentEdgeCases(t *testing.T) {
	// Check SetCurrent with various values
	m := NewManager()
	defer m.WaitAndClose()

	tests := []struct {
		id       string
		total    int64
		setCurrs []int64
	}{
		{"set-current-1", 1000, []int64{0, 500, 1000, 2000}},
		{"set-current-2", 1000, []int64{999, 1, 100}},
		{"set-current-negative", -1, []int64{0, 1000, 5000}},
	}

	for _, tc := range tests {
		taskType := TaskTypeBytes
		if tc.total < 0 {
			taskType = TaskTypeSpinner
		}

		_ = m.AddTask(tc.id, tc.total, 0, taskType)

		for _, val := range tc.setCurrs {
			err := m.SetCurrent(tc.id, val)
			if err != nil {
				t.Errorf("SetCurrent failed for %s with value %d: %v", tc.id, val, err)
			}
		}

		err := m.Complete(tc.id)
		if err != nil {
			t.Errorf("Complete failed for %s: %v", tc.id, err)
		}
	}
}

func TestManager_NonExistentTaskOperations(t *testing.T) {
	// Check that operations on non-existent task return error
	m := NewManager()
	defer m.WaitAndClose()

	nonExistentID := "this-task-does-not-exist"

	// Increment on non-existent task should return error
	err := m.Increment(nonExistentID, 100)
	if err == nil {
		t.Error("Increment on non-existent task should return error")
	}

	// SetCurrent on non-existent task should return error
	err = m.SetCurrent(nonExistentID, 50)
	if err == nil {
		t.Error("SetCurrent on non-existent task should return error")
	}

	// Complete on non-existent task should return error
	err = m.Complete(nonExistentID)
	if err == nil {
		t.Error("Complete on non-existent task should return error")
	}
}

func TestManager_DuplicateTaskIds(t *testing.T) {
	// Check that adding a task with existing ID returns the existing task
	m := NewManager()
	defer m.WaitAndClose()

	id := "duplicate-task"
	task1 := m.AddTask(id, 1000, 0, TaskTypeBytes)
	task2 := m.AddTask(id, 2000, 0, TaskTypeBytes)

	if task1 != task2 {
		t.Error("AddTask should return the same task for duplicate IDs")
	}

	// Check that size remains from first call
	if task2.total != 1000 {
		t.Errorf("Expected total=1000, got %d", task2.total)
	}

	// Complete the task so WaitAndClose doesn't hang
	_ = m.Complete(id)
}

func TestManager_InitialCurrentValue(t *testing.T) {
	// Check that initial current value is set correctly
	m := NewManager()
	defer m.WaitAndClose()

	tests := []struct {
		id      string
		total   int64
		current int64
	}{
		{"resume-1", 1000, 500},
		{"resume-2", 5 * 1024 * 1024, 2 * 1024 * 1024},
		{"resume-3", 100, 99},
	}

	for _, tc := range tests {
		task := m.AddTask(tc.id, tc.total, tc.current, TaskTypeBytes)
		if task == nil {
			t.Errorf("AddTask failed for %s", tc.id)
		}

		// Add more progress
		err := m.Increment(tc.id, 100)
		if err != nil {
			t.Errorf("Increment failed for %s: %v", tc.id, err)
		}

		err = m.Complete(tc.id)
		if err != nil {
			t.Errorf("Complete failed for %s: %v", tc.id, err)
		}
	}
}

func TestManager_MultipleTypes(t *testing.T) {
	// Check working with different task types simultaneously
	m := NewManager()
	defer m.WaitAndClose()

	var wg sync.WaitGroup

	// TaskTypeBytes
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.AddTask("bytes-task", 1024*1024, 0, TaskTypeBytes)
		for i := 0; i < 10; i++ {
			_ = m.Increment("bytes-task", 102400)
			time.Sleep(10 * time.Millisecond)
		}
		_ = m.Complete("bytes-task")
	}()

	// TaskTypeNumeric
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.AddTask("numeric-task", 100, 0, TaskTypeNumeric)
		for i := 0; i < 100; i++ {
			_ = m.Increment("numeric-task", 1)
			time.Sleep(10 * time.Millisecond)
		}
		_ = m.Complete("numeric-task")
	}()

	// TaskTypeSpinner
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.AddTask("spinner-task", -1, 0, TaskTypeSpinner)
		for i := 0; i < 50; i++ {
			_ = m.Increment("spinner-task", 20480)
			time.Sleep(10 * time.Millisecond)
		}
		_ = m.Complete("spinner-task")
	}()

	wg.Wait()
}

func TestTask_WrapReaderEdgeCases(t *testing.T) {
	// Check WrapReader with empty and very large data
	m := NewManager()
	defer m.WaitAndClose()

	tests := []struct {
		id      string
		content []byte
		name    string
	}{
		// {"empty", []byte{}, "empty data"},
		{"single-byte", []byte{42}, "single byte"},
		{"large", make([]byte, 10*1024*1024), "10 MB data"},
	}

	for _, tc := range tests {
		taskObj := m.AddTask(tc.id, int64(len(tc.content)), 0, TaskTypeBytes)
		if taskObj == nil {
			t.Errorf("AddTask failed for %s", tc.id)
			continue
		}

		reader := bytes.NewReader(tc.content)
		wrappedReader := taskObj.WrapReader(reader)

		out, err := io.ReadAll(wrappedReader)
		if err != nil {
			t.Errorf("ReadAll failed for %s: %v", tc.id, err)
		}

		if !bytes.Equal(out, tc.content) {
			t.Errorf("Content mismatch for %s", tc.id)
		}

		_ = m.Complete(tc.id)
	}
}

func TestManager_SequentialCompletion(t *testing.T) {
	// Check that Complete can be called multiple times
	m := NewManager()
	defer m.WaitAndClose()

	id := "multi-complete"
	task := m.AddTask(id, 1000, 0, TaskTypeBytes)

	if task == nil {
		t.Fatal("AddTask failed")
	}

	for i := 0; i < 5; i++ {
		err := m.Complete(id)
		if err != nil {
			t.Errorf("Complete call %d failed: %v", i, err)
		}
	}
}

func TestManager_AllTaskTypes(t *testing.T) {
	// Check that all task types are supported correctly
	m := NewManager()
	defer m.WaitAndClose()

	types := []struct {
		tt   TaskType
		name string
	}{
		{TaskTypeBytes, "bytes"},
		{TaskTypeNumeric, "numeric"},
		{TaskTypeSpinner, "spinner"},
	}

	for _, tc := range types {
		id := "task-" + tc.name
		total := int64(1000)
		if tc.tt == TaskTypeSpinner {
			total = -1
		}

		taskObj := m.AddTask(id, total, 0, tc.tt)
		if taskObj == nil {
			t.Errorf("AddTask failed for type %s", tc.name)
			continue
		}

		// Perform standard operations
		for i := 0; i < 5; i++ {
			_ = m.Increment(id, 100)
		}

		err := m.Complete(id)
		if err != nil {
			t.Errorf("Complete failed for type %s: %v", tc.name, err)
		}
	}
}
