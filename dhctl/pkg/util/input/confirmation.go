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
	"os"

	"github.com/pterm/pterm"
	terminal "golang.org/x/term"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/progressbar"
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
	if !IsTerminal() {
		return c.defaultAnswer
	}

	pb := progressbar.GetDefaultPb()
	confirmWriter := pb.MultiPrinter.NewWriter()
	oldWriter := pb.MultiPrinter.Writer
	pterm.SetDefaultOutput(confirmWriter)
	result, _ := pterm.DefaultInteractiveConfirm.Show(c.message)
	pterm.SetDefaultOutput(oldWriter)

	return result
}

func IsTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}
