package retry

import (
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/tomb"
)

const attemptMessage = `Attempt #%d of %d |
	%s failed, next attempt will be in %ds"
`

var InTestEnvironment = false

func setupTests(attemptsQuantity, waitSeconds *int) {
	if InTestEnvironment {
		*attemptsQuantity = 1
		*waitSeconds = 0
	}
}

func StartLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	setupTests(&attemptsQuantity, &waitSeconds)
	return log.Process("default", name, func() error {
		for i := 1; i <= attemptsQuantity; i++ {
			select {
			case <-tomb.Ctx().Done():
				return fmt.Errorf("loop was canceled")
			default:
				err := task()
				if err == nil {
					log.Success("Succeeded!\n")
					return nil
				}

				log.Fail(fmt.Sprintf(attemptMessage, i, attemptsQuantity, name, waitSeconds))

				log.InfoF("\tError: %v\n\n", err)
				time.Sleep(time.Duration(waitSeconds) * time.Second)
			}
		}
		return fmt.Errorf("loop %q timed out", name)
	})
}

func StartSilentLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	setupTests(&attemptsQuantity, &waitSeconds)
	var err error
	for i := 1; i <= attemptsQuantity; i++ {
		if err = task(); err != nil {
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		}

		return nil
	}
	return fmt.Errorf("timeout while %q: last error: %v", name, err)
}
