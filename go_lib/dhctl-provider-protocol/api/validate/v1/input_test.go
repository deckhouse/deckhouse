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

package v1

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

func TestInputValidate(t *testing.T) {
	tests := []struct {
		name      string
		operation Operation
		wantErr   error
		wantText  string
	}{
		{name: "accepts bootstrap", operation: OperationBootstrap},
		{name: "accepts converge", operation: OperationConverge},
		{name: "accepts destroy", operation: OperationDestroy},
		{
			name:     "rejects missing operation",
			wantErr:  errs.ErrInvalidRequest,
			wantText: "operation required",
		},
		{
			// An operation the validator does not recognise would silently get the
			// checks of some other phase.
			name:      "rejects unknown operation",
			operation: "nonsense",
			wantErr:   errs.ErrInvalidRequest,
			wantText:  `operation unknown: "nonsense"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Input{ProviderName: "dvp", Operation: test.operation}.Validate()

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, test.wantErr)
			}

			if !strings.Contains(err.Error(), test.wantText) {
				t.Errorf("Validate() = %q, want it to mention %q", err, test.wantText)
			}
		})
	}
}

func TestInputToRequest(t *testing.T) {
	tests := []struct {
		name  string
		input Input
	}{
		{
			// The input must survive the wire unchanged: the caller builds it from
			// cluster objects and the validator checks those objects field by field.
			name: "carries every field",
			input: Input{
				ProviderName:          "dvp",
				ClusterPrefix:         "my-cluster",
				Layout:                "Standard",
				Operation:             OperationBootstrap,
				ProviderClusterConfig: map[string]any{"layout": "Standard", "replicas": float64(3)},
				CloudProviderVars: &CloudProviderVars{
					Settings:        map[string]any{"zone": "a"},
					NodeGroups:      map[string]map[string]any{"master": {"kind": "NodeGroup"}},
					InstanceClasses: map[string]map[string]any{"worker": {"kind": "DVPInstanceClass"}},
					Secrets:         map[string]map[string]any{"d8-credentials": {"type": CredentialsSecretType}},
				},
			},
		},
		{
			// Absent vars must stay absent rather than become an empty struct: a
			// validator branches on nil to tell "nothing was collected" from
			// "nothing exists".
			name:  "keeps nil vars nil",
			input: Input{ProviderName: "dvp", Operation: OperationConverge},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := test.input.ToRequest()
			if err != nil {
				t.Fatalf("ToRequest() = %v", err)
			}

			got, err := InputFromRequest(req)
			if err != nil {
				t.Fatalf("InputFromRequest() = %v", err)
			}

			if !reflect.DeepEqual(got, test.input) {
				t.Errorf("round trip = %+v, want %+v", got, test.input)
			}
		})
	}
}

func TestInputFromRequest(t *testing.T) {
	tests := []struct {
		name      string
		inputJSON string
		nilJSON   bool
		want      Input
		wantErr   bool
	}{
		{
			name:      "decodes an input",
			inputJSON: `{"providerName":"dvp","operation":"converge"}`,
			want:      Input{ProviderName: "dvp", Operation: OperationConverge},
		},
		{
			name:      "ignores unknown fields",
			inputJSON: `{"providerName":"dvp","operation":"destroy","future":true}`,
			want:      Input{ProviderName: "dvp", Operation: OperationDestroy},
		},
		{
			name:      "fails on malformed json",
			inputJSON: `{"providerName":`,
			wantErr:   true,
		},
		{
			// Fail closed: an empty payload is a broken caller, not an empty input.
			name:    "fails on missing payload",
			nilJSON: true,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &ValidateRequest{}
			if !test.nilJSON {
				req.InputJson = []byte(test.inputJSON)
			}

			got, err := InputFromRequest(req)

			if test.wantErr {
				if err == nil {
					t.Fatalf("InputFromRequest() = %+v, want an error", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("InputFromRequest() = %v", err)
			}

			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("InputFromRequest() = %+v, want %+v", got, test.want)
			}
		})
	}
}
