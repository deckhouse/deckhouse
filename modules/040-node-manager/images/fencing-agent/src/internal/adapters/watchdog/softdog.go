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
	"strings"
	"sync"
)

type Config struct {
	Device  string `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	Timeout int    `env:"WATCHDOG_TIMEOUT"` // required if FENCING_MODE=watchdog
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Device) == "" {
		return errors.New("WATCHDOG_DEVICE env var is empty")
	}

	return nil
}

type WatchDog struct {
	watchdogDeviceName string
	watchdogDevice     *os.File
	isArmed            bool
	mu                 sync.RWMutex
}

func New(device string) *WatchDog {
	return &WatchDog{
		watchdogDeviceName: device,
		isArmed:            false,
	}
}

func (w *WatchDog) IsArmed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.isArmed
}

func (w *WatchDog) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error
	if w.isArmed {
		return fmt.Errorf("watchdog is already armed")
	}
	w.watchdogDevice, err = os.OpenFile(w.watchdogDeviceName, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("unable to open watchdog device: %w", err)
	}
	w.isArmed = true
	return nil
}

func (w *WatchDog) Feed() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.isArmed {
		return fmt.Errorf("watchdog is not opened")
	}
	_, err := w.watchdogDevice.Write([]byte{'1'})
	if err != nil {
		return fmt.Errorf("unable to feed watchdog: %w", err)
	}
	return nil
}

func (w *WatchDog) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isArmed {
		return fmt.Errorf("watchdog is already closed")
	}
	// Attempt a Magic Close to disarm the watchdog device
	_, err := w.watchdogDevice.Write([]byte{'V'})
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return nil
		}
		return fmt.Errorf("unable to disarm watchdog: %w", err)
	}
	err = w.watchdogDevice.Close()
	if err != nil {
		return fmt.Errorf("unable to close watchdog device: %w", err)
	}
	w.isArmed = false
	return nil
}
