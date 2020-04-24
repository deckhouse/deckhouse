package frontend

import (
	"fmt"
	"time"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/session"
)

const (
	ConnectionAttemptsCount = 35
	ConnectionAttemptDelay  = 5 * time.Second
)

type Check struct {
	Session *session.Session
}

func NewCheck(sess *session.Session) *Check {
	return &Check{Session: sess}
}

func (c *Check) AwaitAvailability() error {
	attempts := 0
	for {
		attempts++
		output, err := c.ExpectAvailable()
		if err == nil {
			logboek.LogLn("Connected successfully")
			return nil
		}
		logboek.LogInfoF("Wait for connection. Attempt #%d of %d.\n", attempts, ConnectionAttemptsCount)
		logboek.LogInfoLn(string(output))

		if attempts == ConnectionAttemptsCount {
			return fmt.Errorf("host '%s' is not available", app.SshHost)
		}
		logboek.LogInfoF("next attempt in %s\n\n", ConnectionAttemptDelay.String())
		time.Sleep(ConnectionAttemptDelay)
	}
}

func (c *Check) ExpectAvailable() ([]byte, error) {
	cmd := NewCommand(c.Session, "echo SUCCESS").Cmd()
	return cmd.CombinedOutput()
}
