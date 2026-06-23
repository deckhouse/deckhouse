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
	"fmt"
	"os"
)

// Handler implements the server side of the dhctl external provider protocol.
// Set Validate and Prepare to your provider logic, then call Run.
type Handler struct {
	Validate func(ctx context.Context, input PrepareInput) error
	Prepare  func(ctx context.Context, input PrepareInput) (*PrepareResult, error)
}

// Run reads the subcommand from os.Args[1], decodes stdin, dispatches to the
// appropriate handler, and encodes the response to stdout.
//
// Decoding errors and unknown subcommands are returned as Go errors — the caller
// should print them to stderr and exit 1.
//
// Business errors (validation failure, prepare failure) are encoded into the
// response's error field and Run returns nil; the binary exits 0.
func (h *Handler) Run(ctx context.Context) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: %s <validate|prepare>", os.Args[0])
	}

	switch os.Args[1] {
	case "validate":
		if h.Validate == nil {
			return fmt.Errorf("validate subcommand is not implemented by this binary")
		}
		return h.runValidate(ctx)
	case "prepare":
		if h.Prepare == nil {
			return fmt.Errorf("prepare subcommand is not implemented by this binary")
		}
		return h.runPrepare(ctx)
	default:
		return fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}
}

func (h *Handler) runValidate(ctx context.Context) error {
	var req ValidateRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return fmt.Errorf("decode validate request: %w", err)
	}

	resp := ValidateResponse{}
	if err := h.Validate(ctx, req.Input); err != nil {
		resp.Error = err.Error()
	}
	return json.NewEncoder(os.Stdout).Encode(resp)
}

func (h *Handler) runPrepare(ctx context.Context) error {
	var req PrepareRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return fmt.Errorf("decode prepare request: %w", err)
	}

	result, err := h.Prepare(ctx, req.Input)
	resp := PrepareResponse{}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Result = result
	}
	return json.NewEncoder(os.Stdout).Encode(resp)
}
