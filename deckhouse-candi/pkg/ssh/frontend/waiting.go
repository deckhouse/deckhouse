package frontend

import (
	"fmt"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/session"
	"flant/deckhouse-candi/pkg/util/retry"
)

type Check struct {
	Session *session.Session
}

func NewCheck(sess *session.Session) *Check {
	return &Check{Session: sess}
}

func (c *Check) AwaitAvailability() error {
	return retry.StartLoop("Waiting for SSH connection", 35, 5, func() error {
		output, err := c.ExpectAvailable()
		if err == nil {
			return nil
		}

		logboek.LogInfoF(string(output))
		return fmt.Errorf("host '%s' is not available", app.SshHost)
	})
}

func (c *Check) ExpectAvailable() ([]byte, error) {
	cmd := NewCommand(c.Session, "echo SUCCESS").Cmd()
	return cmd.CombinedOutput()
}
