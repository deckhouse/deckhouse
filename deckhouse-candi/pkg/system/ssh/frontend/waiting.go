package frontend

import (
	"fmt"
	"strings"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh/session"
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

		log.InfoF(string(output))
		return fmt.Errorf("host '%s' is not available", app.SshHost)
	})
}

func (c *Check) ExpectAvailable() ([]byte, error) {
	cmd := NewCommand(c.Session, "echo SUCCESS").Cmd()
	return cmd.CombinedOutput()
}

func (c *Check) String() string {
	builder := strings.Builder{}
	builder.WriteString("ssh ")

	if c.Session.BastionHost != "" {
		builder.WriteString("-J ")
		if c.Session.BastionUser != "" {
			builder.WriteString(fmt.Sprintf("%s@%s", c.Session.User, c.Session.Host))
		} else {
			builder.WriteString(c.Session.Host)
		}
		if c.Session.BastionPort != "" {
			builder.WriteString(fmt.Sprintf(":%s", c.Session.BastionPort))
		}
		builder.WriteString(" ")
	}

	if c.Session.User != "" {
		builder.WriteString(fmt.Sprintf("%s@%s", c.Session.User, c.Session.Host))
	} else {
		builder.WriteString(c.Session.Host)
	}

	if c.Session.Port != "" {
		builder.WriteString(fmt.Sprintf(":%s", c.Session.Port))
	}

	return builder.String()
}
