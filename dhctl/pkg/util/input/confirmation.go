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
	"context"
	"fmt"
	"os"
	"strings"

	terminal "golang.org/x/term"

	logger "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
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

	// Ask() is called from many places without a context, and its signature is part of a widely
	// used exported API, so we use a background context here. Pause/Resume progress markers are
	// best-effort: when SetDefault(root) is wired in production, FromContext routes them to the
	// active terminal session; when no session is active they are inert.
	ctx := context.Background()
	l := logger.FromContext(ctx)

	// Stop the progress UI (if any) so the prompt and typed input are not clobbered by a redraw.
	logger.PauseProgress(ctx, l)
	defer logger.ResumeProgress(ctx, l)

	reader := bufio.NewReader(os.Stdin)
	for {
		l.WarnContext(ctx, fmt.Sprintf("%s [y/n]: ", c.message))
		line, _, err := reader.ReadLine()
		if err != nil {
			l.ErrorContext(ctx, fmt.Sprintf("can't read from stdin: %v", err))
			return false
		}

		response := strings.ToLower(strings.TrimSpace(string(line)))

		switch response {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

func IsTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}
