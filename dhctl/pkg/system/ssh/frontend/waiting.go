package frontend

import (
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
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
	return retry.NewLoop("Waiting for SSH connection", 35, 5*time.Second).Run(func() error {
		log.InfoF("Try to connect to %v host\n", c.Session.Host())
		output, err := c.ExpectAvailable()
		if err == nil {
			return nil
		}

		log.InfoF(string(output))
		oldHost := c.Session.Host()
		c.Session.ChoiceNewHost()
		return fmt.Errorf("host '%s' is not available", oldHost)
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
