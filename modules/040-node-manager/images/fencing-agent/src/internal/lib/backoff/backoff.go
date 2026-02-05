package backoff

import (
	"context"
	"fencing-agent/internal/lib/logger/sl"
	"fmt"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func Wrap(ctx context.Context, log *log.Logger, triesLimit int, moduleName string, f func() error) func() error {
	return func() error {
		err := f()

		currenTry := 1

		base, mx := time.Second, time.Minute

		for backoff := base; err != nil; backoff <<= 1 {
			if currenTry > triesLimit {
				return fmt.Errorf("failed to start %s, tries limit reached: %w", moduleName, err)
			}

			if backoff > mx {
				backoff = mx
			}
			log.Warn("failed to start module", slog.String("module_name", moduleName), sl.Err(err), slog.String("backoff", backoff.String()), slog.Int("tries", currenTry))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			err = f()

			currenTry++
		}
		return err
	}

}
