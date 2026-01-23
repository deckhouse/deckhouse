package fencingconfig

import (
	"errors"
	"strings"
	"time"
)

type WatchdogConfig struct {
	WatchdogDevice       string        `env:"WATCHDOG_DEVICE" env-default:"/dev/watchdog"`
	WatchdogFeedInterval time.Duration `env:"WATCHDOG_FEED_INTERVAL" env-default:"5s"`
}

func (wc *WatchdogConfig) validate() error {
	if strings.TrimSpace(wc.WatchdogDevice) == "" {
		return errors.New("WATCHDOG_DEVICE env var is empty")
	}
	return nil
}
