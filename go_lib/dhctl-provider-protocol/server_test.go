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
		`{"version":"1","input":{"providerName":"dvp"}}`,
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

func TestHandlerRunRejectsUnsupportedVersion(t *testing.T) {
	_, err := runHandlerWithStdio(
		t,
		[]string{"validator", "validate"},
		`{"version":"2","input":{}}`,
		func() error {
			handler := Handler{
				Validate: func(context.Context, PrepareInput) error {
					t.Fatalf("Validate must not be called for unsupported version")
					return nil
				},
			}
			return handler.Run(context.Background())
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol version") {
		t.Fatalf("Handler.Run() error = %v, want unsupported protocol version", err)
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
