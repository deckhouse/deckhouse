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
	if c.Session.Host() == "" {
		return fmt.Errorf("Empty host for connection received")
	}
	time.Sleep(c.delay)
	return retry.NewLoop("Waiting for SSH connection", 50, 5*time.Second).Run(func() error {
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

func (c *Check) CheckAvailability() error {
	if c.Session.Host() == "" {
		return fmt.Errorf("empty host for connection received")
	}

	log.InfoF("Try to connect to %v host\n", c.Session.Host())
	output, err := c.ExpectAvailable()
	if err != nil {
		log.InfoF(string(output))
		return err
	}
	return nil
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

	return output, fmt.Errorf("SSH command output should contain \"SUCCESS\", error: %v", err)
}

func (c *Check) String() string {
	return c.Session.String()
}
