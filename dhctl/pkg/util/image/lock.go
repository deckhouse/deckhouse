// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	lockPollInterval = 100 * time.Millisecond
	// Those who gave up on a stuck holder line up behind the next lock file, so a stuck holder
	// costs one more download, not one per waiter.
	lockFiles = 2
)

// lockWait is how long one lock file is waited for. A var so tests can shorten it.
var lockWait = 2 * time.Minute

// LockDir serialises work on dir between goroutines and dhctl processes sharing a download root and
// reports whether it holds the first lock file. The kernel drops a dead holder's lock; a stuck holder
// is waited for lockFiles times lockWait and a failing lock is skipped, so callers must not rely on it.
func LockDir(ctx context.Context, dir string) (func() error, bool) {
	for n := range lockFiles {
		path := fmt.Sprintf("%s.lock.%d", dir, n)
		unlock, locked, err := lockFile(ctx, path)
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Lock %s cannot be taken, going on without it: %v", path, err))
			break
		}
		if locked {
			return unlock, n == 0
		}
	}

	return func() error { return nil }, false
}

func lockFile(ctx context.Context, path string) (func() error, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open lock file %s: %w", path, err)
	}

	deadline := time.Now().Add(lockWait)
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() error { return closeLockFile(file) }, true, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			return nil, false, errors.Join(fmt.Errorf("lock %s: %w", path, err), closeLockFile(file))
		}
		if time.Now().After(deadline) {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Lock %s is still held after %s, going on without it", path, lockWait))
			return nil, false, closeLockFile(file)
		}

		select {
		case <-ctx.Done():
			return nil, false, closeLockFile(file)
		case <-time.After(lockPollInterval):
		}
	}
}

func closeLockFile(file *os.File) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close lock file %s: %w", file.Name(), err)
	}

	return nil
}
