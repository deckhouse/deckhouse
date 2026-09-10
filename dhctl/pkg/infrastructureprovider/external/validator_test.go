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
	"time"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    []fakeOption
		input   config.ProviderInput
		wantErr []string
	}{
		{
			name:  "passes a valid configuration",
			input: convergeInput(),
		},
		{
			// The caller waits for the announcement rather than dialling early.
			name:  "waits for a validator that takes a while to listen",
			opts:  []fakeOption{withListenAfter(300 * time.Millisecond)},
			input: convergeInput(),
		},
		{
			name:    "reports violations as the error text",
			opts:    []fakeOption{withViolations()},
			input:   convergeInput(),
			wantErr: []string{"Secret/d8-credentials: credential Secret is required"},
		},
		{
			// Warnings are for the operator to read; only errors block.
			name:  "a warning alone does not block the operation",
			opts:  []fakeOption{withWarnings()},
			input: convergeInput(),
		},
		{
			// Fail closed: the violation being there at all is what blocks, not the
			// text it renders to.
			name:    "fails closed on a violation with no detail",
			opts:    []fakeOption{withBlankViolation()},
			input:   convergeInput(),
			wantErr: []string{`provider "dvp" validation failed`},
		},
		{
			// Fail closed: a binary that predates the protocol exits on the unknown
			// subcommand, and the caller learns that rather than waiting out the
			// announcement timeout.
			name:    "fails closed on a binary without the serve subcommand",
			opts:    []fakeOption{withUnknownSubcommand()},
			input:   convergeInput(),
			wantErr: []string{"validator exited: exit status 1"},
		},
		{
			// Fail closed: the endpoint arrived, but the validator was gone before it
			// could answer, and the error says how it went.
			name: "fails closed on a validator that dies after announcing",
			opts: []fakeOption{
				withAnnouncedAddress(unservedAddress(t)),
				withExitAfter(500*time.Millisecond, 3),
			},
			input: convergeInput(),
			wantErr: []string{
				"call validator on",
				"validator exited: exit status 3",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setFakeConfig(t, test.opts...)

			err := Validate(context.Background(), os.Args[0], test.input)

			if len(test.wantErr) == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", test.wantErr)
			}

			for _, want := range test.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Validate() = %q, want it to mention %q", err, want)
				}
			}
		})
	}
}

func convergeInput() config.ProviderInput {
	return config.ProviderInput{ProviderName: "dvp", Operation: string(validatev1.OperationConverge)}
}
