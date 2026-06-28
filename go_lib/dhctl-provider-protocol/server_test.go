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

package dhctlproviderprotocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHandlerRunValidateEncodesBusinessError(t *testing.T) {
	stdout, err := runHandlerWithStdio(
		t,
		[]string{"validator", "validate"},
		`{"input":{"providerName":"dvp"}}`,
		func() error {
			handler := Handler{
				Validate: func(_ context.Context, input PrepareInput) error {
					if input.ProviderName != "dvp" {
						t.Fatalf("Validate input.ProviderName = %q, want dvp", input.ProviderName)
					}
					return errors.New("validation failed")
				},
			}
			return handler.Run(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("Handler.Run() error = %v", err)
	}

	var response ValidateResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != "validation failed" {
		t.Fatalf("ValidateResponse.Error = %q, want validation failed", response.Error)
	}
}

func TestHandlerRunValidateEncodesSuccess(t *testing.T) {
	stdout, err := runHandlerWithStdio(
		t,
		[]string{"validator", "validate"},
		`{"input":{"providerName":"dvp","operation":"bootstrap","vars":{"settings":{"region":"ru-1"}}}}`,
		func() error {
			handler := Handler{
				Validate: func(_ context.Context, input PrepareInput) error {
					if input.Operation != OperationBootstrap {
						t.Fatalf("Validate input.Operation = %q, want bootstrap", input.Operation)
					}
					if input.Vars == nil || input.Vars.Settings["region"] != "ru-1" {
						t.Fatalf("Validate input.Vars.Settings = %#v, want region", input.Vars)
					}
					return nil
				},
			}
			return handler.Run(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("Handler.Run() error = %v", err)
	}

	var response ValidateResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != "" {
		t.Fatalf("ValidateResponse.Error = %q, want empty", response.Error)
	}
}

func TestHandlerRunPrepareEncodesResult(t *testing.T) {
	stdout, err := runHandlerWithStdio(
		t,
		[]string{"validator", "prepare"},
		`{"input":{"providerName":"dvp","providerClusterConfiguration":{"layout":{"zones":["a"]}},"vars":{"nodeGroups":{"worker":{"metadata":{"name":"worker"}}}}}}`,
		func() error {
			handler := Handler{
				Prepare: func(_ context.Context, input PrepareInput) (*PrepareResult, error) {
					if input.Vars == nil || input.Vars.NodeGroups["worker"]["metadata"] == nil {
						t.Fatalf("Prepare input.Vars.NodeGroups = %#v, want worker", input.Vars)
					}
					return &PrepareResult{
						Vars: input.Vars,
						ProviderClusterConfig: map[string]interface{}{
							"layout": map[string]interface{}{"zones": []interface{}{"a", "b"}},
						},
					}, nil
				},
			}
			return handler.Run(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("Handler.Run() error = %v", err)
	}

	var response PrepareResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != "" {
		t.Fatalf("PrepareResponse.Error = %q, want empty", response.Error)
	}
	if response.Result == nil || response.Result.Vars == nil {
		t.Fatalf("PrepareResponse.Result = %#v, want vars", response.Result)
	}
	zones, ok := response.Result.ProviderClusterConfig["layout"].(map[string]interface{})["zones"].([]interface{})
	if !ok || len(zones) != 2 || zones[1] != "b" {
		t.Fatalf("PrepareResponse.Result.ProviderClusterConfig = %#v, want replaced zones", response.Result.ProviderClusterConfig)
	}
}

func TestHandlerRunPrepareEncodesBusinessError(t *testing.T) {
	stdout, err := runHandlerWithStdio(
		t,
		[]string{"validator", "prepare"},
		`{"input":{"providerName":"dvp"}}`,
		func() error {
			handler := Handler{
				Prepare: func(context.Context, PrepareInput) (*PrepareResult, error) {
					return nil, errors.New("prepare failed")
				},
			}
			return handler.Run(context.Background())
		},
	)
	if err != nil {
		t.Fatalf("Handler.Run() error = %v", err)
	}

	var response PrepareResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Error != "prepare failed" {
		t.Fatalf("PrepareResponse.Error = %q, want prepare failed", response.Error)
	}
	if response.Result != nil {
		t.Fatalf("PrepareResponse.Result = %#v, want nil", response.Result)
	}
}

func TestHandlerRunRejectsInvalidDispatch(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		handler Handler
		want    string
	}{
		{
			name: "missing subcommand",
			args: []string{"validator"},
			want: "usage: validator <validate|prepare>",
		},
		{
			name: "unknown subcommand",
			args: []string{"validator", "check"},
			want: "unknown subcommand: check",
		},
		{
			name: "validate handler missing",
			args: []string{"validator", "validate"},
			want: "validate subcommand is not implemented by this binary",
		},
		{
			name: "prepare handler missing",
			args: []string{"validator", "prepare"},
			want: "prepare subcommand is not implemented by this binary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, err := runHandlerWithStdio(t, tt.args, `{}`, func() error {
				return tt.handler.Run(context.Background())
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Handler.Run() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestHandlerRunRejectsMalformedJSON(t *testing.T) {
	_, err := runHandlerWithStdio(
		t,
		[]string{"validator", "validate"},
		`{`,
		func() error {
			handler := Handler{Validate: func(context.Context, PrepareInput) error { return nil }}
			return handler.Run(context.Background())
		},
	)
	if err == nil || !strings.Contains(err.Error(), "decode validate request") {
		t.Fatalf("Handler.Run() error = %v, want decode validate request", err)
	}
}

func runHandlerWithStdio(
	t *testing.T,
	args []string,
	stdin string,
	run func() error,
) (string, error) {
	t.Helper()

	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	})

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer inR.Close()
	defer outR.Close()

	if _, err := inW.WriteString(stdin); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := inW.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	os.Args = args
	os.Stdin = inR
	os.Stdout = outW

	runErr := run()
	if err := outW.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	return string(output), runErr
}
