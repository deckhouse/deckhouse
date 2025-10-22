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

package input

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	terminal "golang.org/x/term"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Confirmation struct {
	message       string
	defaultAnswer bool
}

func NewConfirmation() *Confirmation {
	return &Confirmation{message: "Should we proceed?"}
}

func (c *Confirmation) WithYesByDefault() *Confirmation {
	c.defaultAnswer = true
	return c
}

func (c *Confirmation) WithMessage(m string) *Confirmation {
	c.message = m
	return c
}

func (c *Confirmation) Ask() bool {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return c.defaultAnswer
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		log.WarnF(fmt.Sprintf("%s [y/n]: ", c.message))
		line, _, err := reader.ReadLine()
		if err != nil {
			log.ErrorF("can't read from stdin: %v\n", err)
			return false
		}

		response := strings.ToLower(strings.TrimSpace(string(line)))

		if response == "y" || response == "yes" {
			log.InfoF("\r")
			return true
		} else if response == "n" || response == "no" {
			log.InfoF("\r")
			return false
		}
		log.InfoF("\r")
	}
}
