/*
Copyright 2026 Flant JSC

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

package watchdog

import (
	"errors"
	"time"
)

// ErrNotArmed is returned by every method called after MagicClose.
var ErrNotArmed = errors.New("watchdog device is not armed")

// Device is the minimal watchdog API the fencing agent needs. See the package
// documentation for the kernel semantics behind each method.
//
// The capability predicates are answered from a single WDIOC_GETSUPPORT read
// performed at Open, so they never fail and never change while armed.
type Device interface {
	// Identity is the driver name, "Software Watchdog" for softdog.
	Identity() string
	// SetTimeoutSupported reports WDIOF_SETTIMEOUT.
	SetTimeoutSupported() bool
	// MagicCloseSupported reports WDIOF_MAGICCLOSE.
	MagicCloseSupported() bool
	// KeepAlive resets the timer.
	KeepAlive() error
	// SetTimeout requests a new timeout, rounded up to whole seconds because the
	// ioctl takes an int, and returns the value the driver accepted.
	SetTimeout(timeout time.Duration) (time.Duration, error)
	// GetTimeout returns the timeout currently in effect.
	GetTimeout() (time.Duration, error)
	// GetTimeLeft is diagnostics only and returns errors.ErrUnsupported on
	// drivers without a get_timeleft op, softdog among them.
	GetTimeLeft() (time.Duration, error)
	// MagicClose disarms the watchdog and closes the device; it is idempotent.
	MagicClose() error
	// ReleaseWithoutDisarm closes the device and leaves the timer running, which
	// is what the kernel does for any process that dies while holding it. It
	// exists to recover a stale descriptor (a reloaded module, an I/O error)
	// without opening a window in which the Node is not fenced.
	ReleaseWithoutDisarm() error
}
