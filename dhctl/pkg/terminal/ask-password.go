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
	"io"
	"os"

	terminal "golang.org/x/term"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// AskBecomePassword reads a sudo/become password from stdin and stores it in o.BecomePass
// when o.AskBecomePass is set. No-op otherwise.
func AskBecomePassword(o *options.BecomeOptions) error {
	if o == nil || !o.AskBecomePass {
		return nil
	}

	data, err := readPassword("[sudo] Password: ")
	if err != nil {
		return err
	}

	o.BecomePass = string(data)
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

// AskBastionPassword reads a bastion password from stdin and stores it in o.BastionPass.
//
// The prompt is skipped when AskBastionPass is unset, when legacy mode is forced,
// or when private keys are configured and modern mode is not forced (matching the
// previous package-global behavior).
func AskBastionPassword(o *options.SSHOptions) error {
	if o == nil || !o.AskBastionPass || o.LegacyMode || (len(o.PrivateKeys) > 0 && !o.ModernMode) {
		return nil
	}

	data, err := readPassword("[bastion] Password: ")
	if err != nil {
		return err
	}

	o.BastionPass = string(data)
	return nil
}

func readPassword(prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())

	var data []byte
	var err error

	if !terminal.IsTerminal(fd) {
		data, err = io.ReadAll(os.Stdin)
	} else {
		log.InfoF(prompt)
		data, err = terminal.ReadPassword(fd)
	}

	log.InfoLn()

	if err != nil {
		return nil, fmt.Errorf("read password: %v", err)
	}
	return data, nil
}
