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
	"context"
	"fmt"
	"os"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"golang.org/x/term"
)

// AskPassword reads a secret from the terminal on stdin without echo. The prompt goes through the
// logger, shown in compact output too, so the user sees what is asked for.
func AskPassword(ctx context.Context, prompt string) ([]byte, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("stdin is not a terminal, cannot read password")
	}

	logger := dhlog.FromContext(ctx)
	logger.InfoContext(ctx, prompt, dhlog.ShowInCompacted())

	data, err := term.ReadPassword(fd)

	logger.InfoContext(ctx, "", dhlog.ShowInCompacted())
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	return data, nil
}
