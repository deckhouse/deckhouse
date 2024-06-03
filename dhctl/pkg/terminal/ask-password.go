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

package terminal

import (
	"fmt"
	"os"

	terminal "golang.org/x/term"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func AskBecomePassword() (err error) {
	if !app.AskBecomePass {
		return nil
	}

	fd := int(os.Stdin.Fd())

	var data []byte
	if !terminal.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal, error reading password")
	}

	log.InfoF("[sudo] Password: ")

	data, err = terminal.ReadPassword(fd)
	log.InfoLn()

	if err != nil {
		return fmt.Errorf("read password: %v", err)
	}

	app.BecomePass = string(data)
	return nil
}

func AskPassword(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())

	if !terminal.IsTerminal(fd) {
		return nil, fmt.Errorf("stdin is not a terminal, error reading password")
	}

	log.InfoF(prompt)
	data, err := terminal.ReadPassword(fd)
	log.InfoLn()

	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}

	return data, nil
}
