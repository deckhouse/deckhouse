/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsBusy(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	assert.False(t, cfg.IsBusy(), "expected IsBusy to be false")

	// Lock the configuration to make IsBusy return true
	done := make(chan struct{})
	go func() {
		cfg.tryToLock(1, 1)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // Small sleep to ensure goroutine starts and acquires lock

	assert.True(t, cfg.IsBusy(), "expected IsBusy to be true")

	<-done // Wait for the goroutine to finish
}

func TestWaitIsBusy(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	// Should not be busy initially
	assert.False(t, cfg.WaitIsBusy(1), "expected WaitIsBusy to be false")

	// Lock the configuration to make WaitIsBusy return true
	done := make(chan struct{})
	go func() {
		cfg.tryToLock(2, 2)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // Small sleep to ensure goroutine starts and acquires lock

	assert.True(t, cfg.WaitIsBusy(1), "expected WaitIsBusy to be true")

	<-done // Wait for the goroutine to finish
}

func TestTryRunFunc(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	ran, err := cfg.tryRunFunc(func() error {
		return nil
	})

	assert.True(t, ran, "expected tryRunFunc to run the function")
	assert.NoError(t, err, "expected no error from tryRunFunc")

	// Lock the configuration to make tryRunFunc return false
	done := make(chan struct{})
	go func() {
		cfg.tryToLock(1, 1)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // Small sleep to ensure goroutine starts and acquires lock

	ran, err = cfg.tryRunFunc(func() error {
		return nil
	})

	assert.False(t, ran, "expected tryRunFunc to not run the function")
	assert.NoError(t, err, "expected no error from tryRunFunc")

	<-done // Wait for the goroutine to finish
}

func TestTryRunFuncWithWait(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	ran, err := cfg.tryRunFuncWithWait(1, func() error {
		return nil
	})

	assert.True(t, ran, "expected tryRunFuncWithWait to run the function")
	assert.NoError(t, err, "expected no error from tryRunFuncWithWait")

	// Lock the configuration to make tryRunFuncWithWait return false after waiting
	done := make(chan struct{})
	go func() {
		cfg.tryToLock(2, 2)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond) // Small sleep to ensure goroutine starts and acquires lock

	ran, err = cfg.tryRunFuncWithWait(1, func() error {
		return nil
	})

	assert.False(t, ran, "expected tryRunFuncWithWait to not run the function")
	assert.NoError(t, err, "expected no error from tryRunFuncWithWait")

	<-done // Wait for the goroutine to finish
}

func TestCreateSingleRequestConfig(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	assert.NotNil(t, cfg, "expected CreateSingleRequestConfig to return a non-nil config")
	assert.False(t, cfg.IsBusy(), "expected new config to not be busy")
}
