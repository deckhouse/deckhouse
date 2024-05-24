/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"time"
)

type SingleRequestConfig struct {
	requestCh chan struct{}
}

func (cfg *SingleRequestConfig) IsBusy() bool {
	isBusy := true
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()
		isBusy = false
	default:
		isBusy = true
	}
	return isBusy
}

func (cfg *SingleRequestConfig) WaitIsBusy(waitSeconds int) bool {
	timeout := time.After(time.Duration(waitSeconds * int(time.Second)))
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()
		return false
	case <-timeout:
		return true
	}
}

func (cfg *SingleRequestConfig) tryRunFunc(f func() error) (bool, error) {
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()
		return true, f()
	default:
		return false, nil
	}
}

func (cfg *SingleRequestConfig) tryRunFuncWithWait(waitSeconds int, f func() error) (bool, error) {
	timeout := time.After(time.Duration(waitSeconds * int(time.Second)))
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()
		return true, f()
	case <-timeout:
		return false, nil
	}
}

// hack for test
func (cfg *SingleRequestConfig) tryToLock(waitSeconds, lockSeconds int) {
	timeout := time.After(time.Duration(waitSeconds * int(time.Second)))
	select {
	case <-cfg.requestCh:
		defer func() {
			cfg.requestCh <- struct{}{}
		}()
		time.Sleep(time.Duration(lockSeconds * int(time.Second)))
		return
	case <-timeout:
		return
	}
}

func CreateSingleRequestConfig() *SingleRequestConfig {
	cfg := &SingleRequestConfig{
		requestCh: make(chan struct{}, 1),
	}
	cfg.requestCh <- struct{}{}
	return cfg
}
