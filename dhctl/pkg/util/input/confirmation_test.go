// Copyright 2026 Flant JSC
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
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	logger "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// Without a terminal the default stands in for an answer, and it used to do so in silence:
// the compact output shows Warn and above, so a run that dropped its work over a question
// nobody was asked looked like a run with nothing to do. Tests have no terminal, which is
// the case under test.
func TestAskWithoutTerminalSaysSo(t *testing.T) {
	tests := []struct {
		name         string
		confirmation *Confirmation
		answer       bool
		logged       string
	}{
		{
			name:         "no by default",
			confirmation: NewConfirmation().WithMessage("Do you want to CHANGE objects state in the cloud?"),
			answer:       false,
			logged:       `Do you want to CHANGE objects state in the cloud? [y/n]: no terminal to ask on, answered \"no\".`,
		},
		{
			name:         "yes by default",
			confirmation: NewConfirmation().WithMessage("Do you want to continue?").WithYesByDefault(),
			answer:       true,
			logged:       `Do you want to continue? [y/n]: no terminal to ask on, answered \"yes\".`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			previous := slog.Default()
			slog.SetDefault(logger.NewBufferLogger(&out))
			t.Cleanup(func() { slog.SetDefault(previous) })

			require.Equal(t, tc.answer, tc.confirmation.Ask())
			require.Contains(t, out.String(), tc.logged)
			require.Contains(t, out.String(), "level=WARN")
		})
	}
}
