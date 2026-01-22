# Progress Bar Package

A sophisticated progress bar management package for Deckhouse that handles multi-threaded dependency loading with Docker-style output, similar to Docker's pull/push operations.

## Features

### Core Features

- **Manager-based Architecture**: Abstraction layer through `Manager` and `Task` structures that wrap `mpb.v8` library
- **Smart Resume Support**: Handles partial downloads correctly with `current > 0` parameter
- **TTY Detection**: Automatic detection of terminal availability and CI environment variables for safe logging
- **Unknown Download Sizes**: Spinner mode for files with unknown total size (`total = -1`)
- **Docker-style UI**:
  - Left: Layer/task ID
  - Center: Compact progress bar
  - Right: Percentage and byte counters
- **io.Reader Integration**: Transparent progress tracking via `WrapReader()` method
- **Thread-safe Operations**: Concurrent-safe manager for parallel downloads
- **Pause/Resume**: Ability to pause and resume all bars simultaneously

### No-op Mode

When TTY is not available or `CI=true` environment variable is set, the manager operates in no-op mode:
- All operations are valid but produce no output
- Prevents spam in CI/CD logs
- Allows same code to work in all environments

## Usage

### Basic Usage

```go
package main

import (
	"github.com/deckhouse/deckhouse/pkg/progressbar"
)

func main() {
	// Create a new manager
	m := progressbar.NewManager()
	defer m.WaitAndClose()

	// Create a task for a 1MB download
	task := m.AddTask("docker-layer-1", 1024*1024, 0)

	// Simulate downloading in chunks
	for i := 0; i < 10; i++ {
		m.Increment("docker-layer-1", 102400)
		// ... actual download work ...
	}

	m.Complete("docker-layer-1")
}
```

### Resume Support

```go
// Resume a partially downloaded file (50% already done)
task := m.AddTask("resume-layer", 1000000, 500000)

// Continue downloading from where we left off
m.Increment("resume-layer", 250000)
m.Increment("resume-layer", 250000)

m.Complete("resume-layer")
```

### Parallel Downloads

```go
import (
	"sync"
	"github.com/deckhouse/deckhouse/pkg/progressbar"
)

func downloadDependencies() {
	m := progressbar.NewManager()
	defer m.WaitAndClose()

	var wg sync.WaitGroup
	
	// Download multiple files in parallel
	layers := []struct {
		id   string
		size int64
	}{
		{"layer1", 5000000},
		{"layer2", 3000000},
		{"layer3", 7000000},
	}

	for _, layer := range layers {
		wg.Add(1)
		go func(l struct{ id string; size int64 }) {
			defer wg.Done()
			
			task := m.AddTask(l.id, l.size, 0)
			// Download logic here
			downloadWithProgress(task)
			m.Complete(l.id)
		}(layer)
	}

	wg.Wait()
}
```

### Transparent io.Copy Integration

```go
import (
	"io"
	"github.com/deckhouse/deckhouse/pkg/progressbar"
)

func downloadFile(dst io.Writer, src io.Reader) error {
	m := progressbar.NewManager()
	defer m.WaitAndClose()

	task := m.AddTask("download.tar.gz", 104857600, 0)
	
	// Wrap reader to track progress automatically
	wrappedSrc := task.WrapReader(src)
	
	_, err := io.Copy(dst, wrappedSrc)
	return err
}
```

### Unknown Size Handling (Spinner Mode)

```go
// When total size is unknown (-1), creates a spinner
task := m.AddTask("streaming-layer", -1, 0)

// Progress can still be tracked even with unknown total
for chunk := range streamingDownload() {
	m.Increment("streaming-layer", int64(len(chunk)))
}
```

## API Reference

### Manager

#### `NewManager() *Manager`
Creates a new progress manager with automatic TTY detection.

#### `AddTask(id string, total, current int64) *Task`
Adds a new progress bar task:
- `id`: Unique identifier for the task (displayed on the left)
- `total`: Total size in bytes (-1 for unknown size spinner mode)
- `current`: Current progress in bytes (for resume support)

Returns the created `Task` or existing task if ID already exists.

#### `Increment(id string, n int64) error`
Advances task progress by n bytes.

#### `SetCurrent(id string, current int64) error`
Sets absolute progress for a task.

#### `Complete(id string) error`
Marks a task as completed (sets current = total).

#### `WaitAndClose() error`
Waits for all bars to complete and closes the manager. Call this when done.

#### `Pause()`
Pauses all progress bars.

#### `Resume()`
Resumes all progress bars.

### Task

#### `WrapReader(r io.Reader) io.Reader`
Wraps an io.Reader to track progress automatically. Perfect for use with `io.Copy()`.

## Output Example

```
layer1_new:              [=====>                              ] 50%  500.0 KB/1.0 MB
layer2_resume:          [=========>                            ] 100%  1.0 MB/1.0 MB [DONE]
layer3_unknown:         [⠙] 
layer4_medium:          [======>                              ] 25%  250.0 KB/1.0 MB
layer5_complete:        [====================================] 100%  2.0 MB/2.0 MB [DONE]
```

## Environment Variables

### `CI=true`
When set, disables progress bars to prevent spam in CI/CD logs.

## Implementation Details

### Thread Safety
- Manager uses a single mutex to protect the task map for safe concurrent access
- All `mpb.Bar` operations are internally thread-safe within the mpb library
- Multiple goroutines can safely call `Increment()`, `SetCurrent()`, and `Complete()` simultaneously without additional per-task locking

### Memory Efficiency
- Uses `mpb.v8` for efficient terminal rendering
- Minimal overhead in no-op mode
- Only one `mpb.Progress` container per manager

### TTY Detection
The package automatically disables bars when:
1. `CI=true` environment variable is set
2. stderr is not a TTY (e.g., piped output)

**Important:** Progress bars are rendered to stderr, not stdout, allowing output redirection (e.g., `dh download > lista.txt`) while keeping bars visible on the terminal.

## Testing

Run the test suite with:

```bash
go test -v ./pkg/progressbar
```

Run benchmarks:

```bash
go test -bench=. ./pkg/progressbar
```

### Test Coverage

- Task creation and management
- Resume support for partial downloads
- Unknown size handling (spinner mode)
- Parallel downloads with various scenarios
- Reader wrapping and io.Copy integration
- Error handling for invalid tasks
- No-op mode verification
- Thread safety under concurrent load

## Performance

- `Increment()`: O(1) operation using `bar.IncrInt64()` for proper int64 support
- `SetCurrent()`: O(1) operation using `bar.SetCurrent()`
- `Complete()`: O(1) operation
- Minimal lock contention with only one manager-level mutex (no per-task locks)

## Integration with Deckhouse

This package is designed to be used in Deckhouse components that need to:
- Download container images from registries
- Pull multiple layers in parallel
- Display user-friendly progress for long-running operations
- Work seamlessly in both interactive terminals and CI/CD environments

## License

Apache License 2.0 - See LICENSE file in the project root.
