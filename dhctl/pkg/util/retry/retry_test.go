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
	"errors"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestLoop_Run_SuccessOnFirstAttempt(t *testing.T) {
	log.InitLogger("json")
	loop := NewLoop("test loop", 3, 1*time.Second)
	err := loop.Run(func() error {
		return nil
	})
	assert.NoError(t, err)
}

func TestLoop_Run_SuccessAfterRetries(t *testing.T) {
	log.InitLogger("json")
	attempt := 0
	loop := NewLoop("test loop", 3, 1*time.Second)
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

// func TestLoop_Run_Interrupted(t *testing.T) {
// 	log.InitLogger("json")
// 	loop := NewLoop("test loop", 3, 1*time.Second).WithInterruptable(true)
// 	tomb.Interrupt(nil) // Simulate interruption
// 	err := loop.Run(func() error {
// 		return errors.New("error")
// 	})
// 	assert.Error(t, err)
// 	assert.Contains(t, err.Error(), "graceful shutdown")
// }

func TestLoop_Run_BreakIfPredicate(t *testing.T) {
	log.InitLogger("json")
	loop := NewLoop("test loop", 3, 1*time.Second).BreakIf(IsErr(errors.New("break error")))
	err := loop.Run(func() error {
		return errors.New("break error")
	})
	assert.Error(t, err)
	assert.Equal(t, "Timeout while \"test loop\": last error: break error", err.Error())
}
