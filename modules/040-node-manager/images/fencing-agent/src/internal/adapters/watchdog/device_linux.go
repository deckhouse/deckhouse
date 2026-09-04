//go:build linux

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
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// magicCloseChar sets the kernel expect_close flag, the only way user space
	// can stop the timer.
	magicCloseChar = 'V'
	// keepAliveChar is the write-based ping. Any byte works as long as it is not
	// the magic character.
	keepAliveChar = '1'
)

// support is the capability set the driver reports through WDIOC_GETSUPPORT.
type support struct {
	Identity string
	Options  uint32
	// SetTimeout is WDIOF_SETTIMEOUT. Without it the SLA profile timeout cannot
	// reach the device.
	SetTimeout bool
	// KeepAlive is WDIOF_KEEPALIVEPING. Only the ioctl needs it; a write ping
	// always works.
	KeepAlive bool
	// MagicClose is WDIOF_MAGICCLOSE. Without it the device cannot be disarmed,
	// so maintenance keeps feeding instead of stopping.
	MagicClose bool
}

type device struct {
	path   string
	logger *log.Logger

	// support is written once by Open, before the device is shared.
	support support

	// mu serialises the descriptor: the feed loop and the shutdown disarm reach it
	// from different goroutines.
	mu   sync.Mutex
	file *os.File

	fallbackOnce sync.Once
}

// Open arms the watchdog and reads its capabilities. The caller owns the
// returned Device and must MagicClose it to disarm.
func Open(path string, logger *log.Logger) (Device, error) {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open watchdog device %q: %w", path, err)
	}

	d := &device{path: path, logger: logger, file: file}

	caps, err := d.readSupport()
	if err != nil {
		// The open already armed the device, so disarm before reporting: a failed
		// capability read must not cost the Node a panic.
		if closeErr := d.MagicClose(); closeErr != nil {
			logger.Error("disarm watchdog after a failed capability read", "error", closeErr)
		}

		return nil, err
	}

	d.support = caps

	logger.Info("watchdog device armed",
		"device", path,
		"identity", caps.Identity,
		"options", fmt.Sprintf("%#x", caps.Options),
		"set_timeout", caps.SetTimeout,
		"keep_alive_ping", caps.KeepAlive,
		"magic_close", caps.MagicClose,
	)

	return d, nil
}

func (d *device) readSupport() (support, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return support{}, ErrNotArmed
	}

	info, err := unix.IoctlGetWatchdogInfo(int(d.file.Fd()))
	if err != nil {
		return support{}, fmt.Errorf("read capabilities of %q (WDIOC_GETSUPPORT): %w", d.path, translate(err))
	}

	return support{
		Identity:   unix.ByteSliceToString(info.Identity[:]),
		Options:    info.Options,
		SetTimeout: info.Options&unix.WDIOF_SETTIMEOUT != 0,
		KeepAlive:  info.Options&unix.WDIOF_KEEPALIVEPING != 0,
		MagicClose: info.Options&unix.WDIOF_MAGICCLOSE != 0,
	}, nil
}

func (d *device) Identity() string {
	return d.support.Identity
}

func (d *device) SetTimeoutSupported() bool {
	return d.support.SetTimeout
}

func (d *device) MagicCloseSupported() bool {
	return d.support.MagicClose
}

func (d *device) KeepAlive() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return ErrNotArmed
	}

	if d.support.KeepAlive {
		err := unix.IoctlWatchdogKeepalive(int(d.file.Fd()))
		if err == nil {
			return nil
		}

		if !errors.Is(translate(err), errors.ErrUnsupported) {
			return fmt.Errorf("keepalive (WDIOC_KEEPALIVE): %w", err)
		}

		// Odd enough to report, but the write below still keeps the Node alive.
		d.fallbackOnce.Do(func() {
			d.logger.Warn("WDIOC_KEEPALIVE rejected despite WDIOF_KEEPALIVEPING, falling back to write", "error", err)
		})
	}

	if _, err := d.file.Write([]byte{keepAliveChar}); err != nil {
		return fmt.Errorf("keepalive write: %w", err)
	}

	return nil
}

func (d *device) SetTimeout(timeout time.Duration) (time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return 0, ErrNotArmed
	}

	// The ioctl takes whole seconds. Round up, or the Node would fence earlier
	// than the SLA promises.
	seconds := int(timeout / time.Second)
	if timeout%time.Second != 0 {
		seconds++
	}

	if err := unix.IoctlSetPointerInt(int(d.file.Fd()), unix.WDIOC_SETTIMEOUT, seconds); err != nil {
		return 0, fmt.Errorf("set timeout to %ds (WDIOC_SETTIMEOUT): %w", seconds, translate(err))
	}

	// Read back instead of trusting the request: a driver may clamp or round it.
	return d.getTimeout()
}

func (d *device) GetTimeout() (time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return 0, ErrNotArmed
	}

	return d.getTimeout()
}

func (d *device) getTimeout() (time.Duration, error) {
	seconds, err := unix.IoctlGetInt(int(d.file.Fd()), unix.WDIOC_GETTIMEOUT)
	if err != nil {
		return 0, fmt.Errorf("read timeout (WDIOC_GETTIMEOUT): %w", translate(err))
	}

	return time.Duration(seconds) * time.Second, nil
}

func (d *device) GetTimeLeft() (time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return 0, ErrNotArmed
	}

	seconds, err := unix.IoctlGetInt(int(d.file.Fd()), unix.WDIOC_GETTIMELEFT)
	if err != nil {
		return 0, fmt.Errorf("read time left (WDIOC_GETTIMELEFT): %w", translate(err))
	}

	return time.Duration(seconds) * time.Second, nil
}

func (d *device) MagicClose() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Idempotent: the shutdown path and the maintenance path can both reach here.
	if d.file == nil {
		return nil
	}

	file := d.file
	d.file = nil

	// The magic character must land before the close, or the kernel keeps counting
	// and the Node panics anyway.
	_, writeErr := file.Write([]byte{magicCloseChar})
	closeErr := file.Close()

	if writeErr != nil {
		return fmt.Errorf("write magic close character: %w", errors.Join(writeErr, closeErr))
	}

	if closeErr != nil {
		return fmt.Errorf("close watchdog device: %w", closeErr)
	}

	return nil
}

func (d *device) ReleaseWithoutDisarm() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file == nil {
		return nil
	}

	file := d.file
	d.file = nil

	// No magic character on purpose: the kernel keeps counting, so the Node stays
	// fenced while the agent reopens the device.
	if err := file.Close(); err != nil {
		return fmt.Errorf("release watchdog device: %w", err)
	}

	return nil
}

// translate maps "the driver has no such ioctl" onto errors.ErrUnsupported, so
// callers can tell a missing capability from a real failure.
func translate(err error) error {
	if errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("%w: %w", errors.ErrUnsupported, err)
	}

	return err
}
