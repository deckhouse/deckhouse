package frontend

import (
	"fmt"
	"strings"
	"time"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh/session"
	"flant/candictl/pkg/util/retry"
)

type Check struct {
	Session *session.Session
	delay   time.Duration
}

func NewCheck(sess *session.Session) *Check {
	return &Check{Session: sess}
}

func (c *Check) WithDelaySeconds(seconds int) *Check {
	c.delay = time.Duration(seconds) * time.Second
	return c
}

func (c *Check) AwaitAvailability() error {
	<-time.After(c.delay)
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
			builder.WriteString(fmt.Sprintf("%s@%s", c.Session.BastionUser, c.Session.BastionHost))
		} else {
			builder.WriteString(c.Session.BastionHost)
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
