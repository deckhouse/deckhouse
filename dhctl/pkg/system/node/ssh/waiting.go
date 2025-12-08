// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type CommandConsumer func(*session.Session, string) node.Command

type Check struct {
	Session       *session.Session
	createCommand CommandConsumer
	delay         time.Duration
}

func NewCheck(createCommand CommandConsumer, sess *session.Session) *Check {
	return &Check{
		Session:       sess,
		createCommand: createCommand,
	}
}

func (c *Check) WithDelaySeconds(seconds int) node.Check {
	c.delay = time.Duration(seconds) * time.Second
	return c
}

func (c *Check) AwaitAvailability(ctx context.Context) error {
	if c.Session.Host() == "" {
		return fmt.Errorf("empty host for connection received")
	}

	select {
	case <-time.After(c.delay):
	case <-ctx.Done():
		return ctx.Err()
	}

	return retry.NewLoop("Waiting for SSH connection", 50, 5*time.Second).RunContext(ctx, func() error {
		host := c.Session.Host()
		log.InfoF("Try to connect to host: %v\n", host)

		output, err := c.ExpectAvailable(ctx)
		if err == nil {
			log.InfoF("Successfully connected to host: %v\n", host)
			return nil
		}

		log.InfoF("Connection attempt failed to host: %v", host)
		oldHost := host
		c.Session.ChoiceNewHost()

		errMsg := "unknown error"
		if err != nil {
			errMsg = err.Error()
		}

		var detailedErr string
		switch {
		case strings.Contains(errMsg, "timeout"):
			detailedErr = fmt.Sprintf("SSH connection timed out to host '%s'", oldHost)
		case strings.Contains(errMsg, "permission denied"):
			detailedErr = fmt.Sprintf("SSH permission access denied to host '%s'", oldHost)
		case strings.Contains(errMsg, "no route to host"), strings.Contains(errMsg, "host unreachable"):
			detailedErr = fmt.Sprintf("No route to host: '%s' unreachable", oldHost)
		default:
			detailedErr = fmt.Sprintf("Failed to connect to host '%s': %s", oldHost, errMsg)
		}

		return fmt.Errorf("%s\nSSH command output:\n%s", detailedErr, strings.TrimSpace(string(output)))
	})
}

func (c *Check) CheckAvailability(ctx context.Context) error {
	if c.Session.Host() == "" {
		return fmt.Errorf("empty host for connection received")
	}

	log.InfoF("Try to connect to %v host\n", c.Session.Host())
	output, err := c.ExpectAvailable(ctx)
	if err != nil {
		log.InfoF(string(output))
		return err
	}
	return nil
}

func (c *Check) ExpectAvailable(ctx context.Context) ([]byte, error) {
	cmd := c.createCommand(c.Session, "echo SUCCESS")
	cmd.Cmd(ctx)
	output, _, err := cmd.Output(ctx)
	if err != nil {
		return output, err
	}

	if strings.Contains(string(output), "SUCCESS") {
		return nil, nil
	}

	return output, fmt.Errorf("SSH command output should contain \"SUCCESS\", error: %w", err)
}

func (c *Check) String() string {
	return c.Session.String()
}
