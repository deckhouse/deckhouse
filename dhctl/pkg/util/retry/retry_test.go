// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestLoop_Run_SuccessOnFirstAttempt(t *testing.T) {
	log.InitLogger("json")
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.Run(func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestLoop_Run_SuccessAfterRetries(t *testing.T) {
	log.InitLogger("json")
	attempt := 0
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.Run(func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestLoop_Run_BreakIfPredicate(t *testing.T) {
	log.InitLogger("json")
	loop := NewLoop("test loop", 3, 10*time.Millisecond).BreakIf(IsErr(errors.New("break error")))
	err := loop.Run(func() error {
		return errors.New("break error")
	})
	assert.Error(t, err)
	assert.Equal(t, "Timeout while \"test loop\": last error: break error", err.Error())
}

func TestLoop_RunContext_SuccessOnFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.InitLogger("json")
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.RunContext(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestLoop_RunContext_SuccessAfterRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.InitLogger("json")
	attempt := 0
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.RunContext(ctx, func() error {
		attempt++
		if attempt < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attempt)
}

func TestLoop_Run_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.InitLogger("json")
	attempt := 0
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.RunContext(ctx, func() error {
		attempt++
		if attempt > 1 {
			cancel()
		}
		return errors.New("error")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 2, attempt)
}

func TestLoop_Run_DeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	log.InitLogger("json")
	attempt := 0
	loop := NewLoop("test loop", 3, 10*time.Millisecond)
	err := loop.RunContext(ctx, func() error {
		attempt++
		return errors.New("error")
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, attempt)
}
