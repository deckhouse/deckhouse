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

package backoff

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/lib/logger/sl"
)

func Wrap(ctx context.Context, log *log.Logger, triesLimit int, moduleName string, f func() error) func() error {
	return func() error {
		err := f()

		currentTry := 1

		base, mx := time.Second, time.Minute

		for backoff := base; err != nil; backoff <<= 1 {
			if currentTry > triesLimit {
				return fmt.Errorf("failed to start %s, tries limit reached: %w", moduleName, err)
			}

			if backoff > mx {
				backoff = mx
			}
			log.Warn("failed to start module", slog.String("module_name", moduleName), sl.Err(err), slog.String("backoff", backoff.String()), slog.Int("tries", currentTry))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			err = f()

			currentTry++
		}
		return err
	}
}
