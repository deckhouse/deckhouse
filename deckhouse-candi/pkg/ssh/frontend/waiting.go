package frontend

import (
	"fmt"
	"time"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/session"
)

var ConnectionAttemptsCount = 35
var ConnectionAttemptDelay = 5 * time.Second

type Check struct {
	Session *session.Session
}

func NewCheck(sess *session.Session) *Check {
	return &Check{Session: sess}
}

func (c *Check) AwaitAvailability() error {
	err := c.ExpectAvailable()
	if err == nil {
		return nil
	}

	attempts := 0
	for {
		attempts++
		logboek.LogInfoF("--- Wait for connection. Attempt #%d of %d. ---\n", attempts, ConnectionAttemptsCount)
		err = c.ExpectAvailable()
		if err == nil {
			return nil
		}
		if attempts == ConnectionAttemptsCount {
			return fmt.Errorf("host '%s' is not available", app.SshHost)
		}
		logboek.LogInfoF("next attempt in %s\n", ConnectionAttemptDelay.String())
		time.Sleep(ConnectionAttemptDelay)
	}

	return nil
}

func (c *Check) ExpectAvailable() error {
	cmd := NewCommand(c.Session, "echo SUCCESS").Cmd()
	return cmd.Run()
}
