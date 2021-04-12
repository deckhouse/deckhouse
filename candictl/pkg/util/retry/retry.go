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

// TODO Proposal of new design for retry loop.
//
// New options for loop:
// - interruption
// - verbosity
// - run in a go routine to use in async mode?
//
// Example:
// err := NewLoop(name, attempts, waitSec).Verbose().Interruptable().Start(func() {
//    doSomeJob()
// }).Wait()

// StartLoop retries a task function until it succeeded. Number of attempts
// and delay between runs are adjustable.
//
// Features:
// - it is "verbose" loop — it prints messages through logboek.
// - this loop is interruptable by the signal watcher in tomb package.
//
// TODO: non-interruptable behavior should be an option.
func StartLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	setupTests(&attemptsQuantity, &waitSeconds)
	return log.Process("default", name, func() error {
		for i := 1; i <= attemptsQuantity; i++ {
			// Check if process is interrupted.
			if tomb.IsInterrupted() {
				return fmt.Errorf("loop was canceled: graceful shutdown")
			}

			// Run task and return if everything is ok.
			err := task()
			if err == nil {
				log.Success("Succeeded!\n")
				return nil
			}

			log.Fail(fmt.Sprintf(attemptMessage, i, attemptsQuantity, name, waitSeconds))
			log.InfoF("\tError: %v\n\n", err)

			// Do not wait after the last iteration.
			if i < attemptsQuantity {
				time.Sleep(time.Duration(waitSeconds) * time.Second)
			}
		}
		return fmt.Errorf("loop %q timed out", name)
	})
}

// StartSilentLoop retries a task function until it succeeded. Number of attempts
// and delay between runs are adjustable.
//
// Features:
// - it is "silent" loop — no messages are printed through logboek.
// - this loop is not interruptable by the signal watcher in tomb package.
//
// TODO: interruptable behavior should be an option.
func StartSilentLoop(name string, attemptsQuantity, waitSeconds int, task func() error) error {
	setupTests(&attemptsQuantity, &waitSeconds)
	var err error
	for i := 1; i <= attemptsQuantity; i++ {
		// Run task and return if everything is ok.
		err = task()
		if err == nil {
			return nil
		}
		// Do not wait after the last iteration.
		if i < attemptsQuantity {
			time.Sleep(time.Duration(waitSeconds) * time.Second)
		}
	}
	return fmt.Errorf("timeout while %q: last error: %v", name, err)
}
