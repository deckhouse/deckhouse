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

package external

import (
	"context"
	"os"
	"strings"
	"testing"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    mode
		input   config.ProviderInput
		wantErr string
	}{
		{
			name:  "passes a valid configuration",
			mode:  modeValid,
			input: convergeInput(),
		},
		{
			// The caller waits for the announcement rather than dialling early.
			name:  "waits for a validator that takes a while to listen",
			mode:  modeSlowStart,
			input: convergeInput(),
		},
		{
			name:    "reports violations as the error text",
			mode:    modeViolations,
			input:   convergeInput(),
			wantErr: "Secret/d8-credentials: credential Secret is required",
		},
		{
			// Warnings are for the operator to read; only errors block.
			name:  "a warning alone does not block the operation",
			mode:  modeWarnings,
			input: convergeInput(),
		},
		{
			// Fail closed: the violation being there at all is what blocks, not the
			// text it renders to.
			name:    "fails closed on a violation with no detail",
			mode:    modeBlank,
			input:   convergeInput(),
			wantErr: `provider "dvp" validation failed`,
		},
		{
			// Fail closed: a binary that predates the protocol exits on the unknown
			// subcommand, and the caller learns that rather than waiting out the
			// announcement timeout.
			name:    "fails closed on a binary without the serve subcommand",
			mode:    modeLegacy,
			input:   convergeInput(),
			wantErr: "validator exited: exit status 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setFakeConfig(t, fakeConfig{Mode: test.mode})

			err := Validate(context.Background(), os.Args[0], test.input)

			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", test.wantErr)
			}

			if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("Validate() = %q, want it to mention %q", err, test.wantErr)
			}
		})
	}
}

func convergeInput() config.ProviderInput {
	return config.ProviderInput{ProviderName: "dvp", Operation: string(validatev1.OperationConverge)}
}
