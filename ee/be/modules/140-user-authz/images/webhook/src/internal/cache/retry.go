/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"errors"
	"fmt"
	"time"
)

const maxRetries = 10

var ErrMaxRetriesReached = fmt.Errorf("exceeded retry limit")

type retryable func() (retry bool, err error)

// Retry tries executing retryable function several times
func Retry(fn retryable) error {
	var err error
	var cont bool

	attempt := 1
	for {
		cont, err = fn()
		if !cont || err == nil {
			break
		}

		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}

		attempt++
		if attempt > maxRetries {
			return fmt.Errorf("%v: %w", err, ErrMaxRetriesReached)
		}

		time.Sleep(100 * time.Millisecond)
	}
	return err
}
