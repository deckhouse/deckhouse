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

// Device is the part of the watchdog API the agent needs. The capability checks
// come from one WDIOC_GETSUPPORT read at Open, so they never fail and never
// change while the device is armed.
type Device interface {
	// Identity is the driver name, "Software Watchdog" for softdog.
	Identity() string
	// SetTimeoutSupported reports WDIOF_SETTIMEOUT.
	SetTimeoutSupported() bool
	// MagicCloseSupported reports WDIOF_MAGICCLOSE.
	MagicCloseSupported() bool
	// KeepAlive resets the timer.
	KeepAlive() error
	// SetTimeout rounds up to whole seconds (the ioctl takes an int) and returns
	// the value the driver accepted.
	SetTimeout(timeout time.Duration) (time.Duration, error)
	GetTimeout() (time.Duration, error)
	// GetTimeLeft is diagnostics only. Drivers without a get_timeleft op, softdog
	// included, return errors.ErrUnsupported.
	GetTimeLeft() (time.Duration, error)
	// MagicClose disarms the watchdog and closes the device; it is idempotent.
	MagicClose() error
	// ReleaseWithoutDisarm closes the device and leaves the kernel timer running,
	// the same as when a holder dies. It recovers a stale descriptor without
	// leaving the Node unfenced for a moment.
	ReleaseWithoutDisarm() error
}
