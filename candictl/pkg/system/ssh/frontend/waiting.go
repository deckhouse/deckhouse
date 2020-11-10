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
	time.Sleep(c.delay)
	return retry.StartLoop("Waiting for SSH connection", 35, 5, func() error {
		output, err := c.ExpectAvailable()
		if err == nil {
			return nil
		}

		log.InfoF(string(output))
		return fmt.Errorf("host '%s' is not available", app.SSHHost)
	})
}

func (c *Check) ExpectAvailable() ([]byte, error) {
	cmd := NewCommand(c.Session, "echo SUCCESS").Cmd()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}

	if strings.Contains(string(output), "SUCCESS") {
		return nil, nil
	}

	return output, fmt.Errorf("SSH command otput should contain \"SUCCESS\", error: %v", err)
}

func (c *Check) String() string {
	return c.Session.String()
}
