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

package kuberbacauth

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
)

// reloadInterval is how often the files of a reloader are read.
const reloadInterval = 5 * time.Second

// reloader keeps a value built from files and rebuilds it when their content changes,
// so a rotated CA bundle or a renewed certificate is picked up without a restart.
//   - The files are read at most once per reloadInterval.
//   - One caller reads them. The others get the loaded value without waiting, even if the read hangs.
//   - Content that cannot be loaded leaves the previous value in use: the files may be caught
//     halfway through a rotation. It is reported once and not loaded again until the files change.
type reloader[T any] struct {
	// name says what the files hold, for messages: "client CA bundle".
	name  string
	files []string
	// load builds the value from the content of the files, in their order.
	load  func(contents [][]byte) (*T, error)
	clock utilcache.Clock
	log   Logger

	current atomic.Pointer[loaded[T]]
	// nextCheck is the time of the next read. Nil means that the files are read at once.
	nextCheck atomic.Pointer[time.Time]

	// mu is held by the caller that reads the files.
	mu sync.Mutex
	// seen identifies the content of the last read. seenErr is why it was not loaded, if it was not.
	seen    [sha256.Size]byte
	seenErr error
	// reported is the error already logged: a broken file is reported once, not on every read.
	reported string
}

// loaded is a value with a version that changes with every load.
type loaded[T any] struct {
	value   *T
	version uint64
}

// newReloader loads the value for the first time. This load must succeed.
func newReloader[T any](name string, files []string, load func(contents [][]byte) (*T, error),
	clock utilcache.Clock, log Logger) (*reloader[T], error) {
	r := &reloader[T]{name: name, files: files, load: load, clock: clock, log: log}
	if err := r.reload(); err != nil {
		return nil, err
	}
	r.schedule(clock.Now())
	return r, nil
}

// get returns the current value and its version, or nil if nothing is loaded.
func (r *reloader[T]) get() (*T, uint64) {
	if r.due(r.clock.Now()) {
		r.check()
	}
	if current := r.current.Load(); current != nil {
		return current.value, current.version
	}
	return nil, 0
}

func (r *reloader[T]) due(now time.Time) bool {
	next := r.nextCheck.Load()
	if next == nil || !now.Before(*next) {
		return true
	}
	// The clock was set back: the next read would otherwise wait until the clock catches up.
	return next.Sub(now) > reloadInterval
}

func (r *reloader[T]) schedule(now time.Time) {
	next := now.Add(reloadInterval)
	r.nextCheck.Store(&next)
}

// check reads the files unless another caller is reading them.
func (r *reloader[T]) check() {
	if !r.mu.TryLock() {
		return
	}
	defer r.mu.Unlock()

	now := r.clock.Now()
	// Another caller may have read the files while this one was getting the lock.
	if !r.due(now) {
		return
	}
	r.schedule(now)

	err := r.reload()
	switch {
	case err == nil:
		r.reported = ""
	case err.Error() != r.reported:
		r.reported = err.Error()
		r.log.Warn("cannot reload the "+r.name+", using the previous one", slog.String("error", r.reported))
	}
}

// reload reads the files and loads their content unless it is the content of the last read.
func (r *reloader[T]) reload() error {
	contents := make([][]byte, len(r.files))
	sum := sha256.New()
	for i, file := range r.files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read the %s: %w", r.name, err)
		}
		contents[i] = data
		// The length keeps the content of one file apart from the next one.
		sum.Write(binary.BigEndian.AppendUint64(nil, uint64(len(data))))
		sum.Write(data)
	}
	var seen [sha256.Size]byte
	sum.Sum(seen[:0])
	if seen == r.seen {
		return r.seenErr
	}

	value, err := r.load(contents)
	if err != nil {
		err = fmt.Errorf("load the %s: %w", r.name, err)
	}
	r.seen, r.seenErr = seen, err
	if err != nil {
		return err
	}
	next := &loaded[T]{value: value, version: 1}
	if current := r.current.Load(); current != nil {
		next.version = current.version + 1
	}
	r.current.Store(next)
	return nil
}
