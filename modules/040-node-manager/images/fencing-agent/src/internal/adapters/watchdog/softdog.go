package watchdog

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	WatchdogDevice  string `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WathcdogTimeout int    `env:"WATCHDOG_TIMEOUT" env-default:"10"`
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.WatchdogDevice) == "" {
		return errors.New("WATCHDOG_DEVICE env var is empty")
	}
	return nil
}

type WatchDog struct {
	watchdogDeviceName string
	watchdogDevice     *os.File
	isArmed            bool
}

func New(device string) *WatchDog {
	return &WatchDog{
		isArmed:            false,
		watchdogDeviceName: device,
	}
}

func (w *WatchDog) IsArmed() bool {
	return w.isArmed
}

func (w *WatchDog) Start() error {
	var err error
	if w.isArmed {
		return fmt.Errorf("Watchdog is already armed")
	}
	w.watchdogDevice, err = os.OpenFile(w.watchdogDeviceName, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Unable to open watchdog device %w", err)
	}
	w.isArmed = true
	return nil
}

func (w *WatchDog) Feed() error {
	if !w.isArmed {
		return fmt.Errorf("watchdog is not opened")
	}
	_, err := w.watchdogDevice.Write([]byte{'1'})
	if err != nil {
		return fmt.Errorf("unable to feed watchdog %w", err)
	}
	return nil
}

func (w *WatchDog) Stop() error {
	if !w.isArmed {
		return fmt.Errorf("watchdog is already closed")
	}
	// Attempt a Magic Close to disarm the watchdog device
	_, err := w.watchdogDevice.Write([]byte{'V'})
	if err != nil {
		if errors.Is(err, os.ErrClosed) {
			return nil
		}
		return fmt.Errorf("unable to disarm watchdog %w", err)
	}
	err = w.watchdogDevice.Close()
	if err != nil {
		return fmt.Errorf("unable to close watchdog device %w", err)
	}
	w.isArmed = false
	return nil
}
