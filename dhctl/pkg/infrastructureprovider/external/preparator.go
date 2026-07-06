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

// Package external delegates Validate/Prepare to an external provider binary
// via the stdin/stdout JSON protocol (see go_lib/dhctl-provider-protocol).
package external

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	otattribute "go.opentelemetry.io/otel/attribute"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type Preparator struct {
	binaryPath string
}

func NewBinaryPreparator(binaryPath string) *Preparator {
	return &Preparator{binaryPath: binaryPath}
}

func (p *Preparator) Validate(ctx context.Context, input config.ProviderInput) error {
	stdout, err := p.call(ctx, "validate", input)
	if err != nil {
		return err
	}

	// A conformant binary always emits a JSON object ("{}\n" on success).
	// Empty stdout means a broken binary — fail closed instead of silently
	// treating it as validated (matches Prepare).
	var resp proto.ValidateResponse
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return fmt.Errorf("parse validate response: %w", err)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func (p *Preparator) Prepare(ctx context.Context, input config.ProviderInput) (proto.PrepareResult, error) {
	stdout, err := p.call(ctx, "prepare", input)
	if err != nil {
		return proto.PrepareResult{}, err
	}

	var resp proto.PrepareResponse
	if err := json.Unmarshal(stdout, &resp); err != nil {
		return proto.PrepareResult{}, fmt.Errorf("parse prepare response: %w", err)
	}
	if resp.Error != "" {
		return proto.PrepareResult{}, errors.New(resp.Error)
	}
	if resp.Result != nil {
		return *resp.Result, nil
	}
	return proto.PrepareResult{}, nil
}

// call encodes input, runs the binary subcommand and returns its stdout.
// Request/response payloads go to the span and debug log in full — deliberate
// development-stage telemetry. ValidateRequest and PrepareRequest share the
// same wire shape, so one request type serves both subcommands.
func (p *Preparator) call(ctx context.Context, subcommand string, input config.ProviderInput) ([]byte, error) {
	ctx, span := telemetry.StartSpan(ctx, "external."+subcommand)
	defer span.End()
	span.SetAttributes(
		otattribute.String("provider.name", input.ProviderName),
		otattribute.String("provider.binary", p.binaryPath),
		otattribute.String("provider.subcommand", subcommand),
	)

	wireInput, err := toWireInput(input)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", subcommand, err)
	}
	payload, err := json.Marshal(proto.PrepareRequest{Input: wireInput})
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", subcommand, err)
	}

	// The request/response carry provider Vars (decoded credentials,
	// kubeconfigDataBase64, etc.); never attach them to spans or logs.
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("external.%s binary=%s request_bytes=%d", subcommand, p.binaryPath, len(payload)))

	stdout, err := p.run(ctx, subcommand, payload)
	if err != nil {
		return nil, err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("external.%s binary=%s response_bytes=%d", subcommand, p.binaryPath, len(stdout)))
	return stdout, nil
}

// toWireInput converts ProviderInput to the wire format; ProviderClusterConfig
// goes from json.RawMessage to plain values.
func toWireInput(input config.ProviderInput) (proto.PrepareInput, error) {
	pcc := make(map[string]interface{}, len(input.ProviderClusterConfig))
	for k, v := range input.ProviderClusterConfig {
		var val interface{}
		if err := json.Unmarshal(v, &val); err != nil {
			return proto.PrepareInput{}, fmt.Errorf("unmarshal provider cluster config key %q: %w", k, err)
		}
		pcc[k] = val
	}

	return proto.PrepareInput{
		ProviderName:          input.ProviderName,
		ClusterPrefix:         input.ClusterPrefix,
		Layout:                input.Layout,
		Operation:             input.Operation,
		ProviderClusterConfig: pcc,
		Vars:                  input.CloudProviderVars,
	}, nil
}

// runTimeout caps one validator invocation so a hung binary cannot hang the caller.
const runTimeout = 30 * time.Second

func (p *Preparator) run(ctx context.Context, subcommand string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.binaryPath, subcommand)
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("provider binary %s: %w\n%s", subcommand, err, stderr.String())
		}
		return nil, fmt.Errorf("provider binary %s: %w", subcommand, err)
	}

	if stderr.Len() > 0 {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("provider binary stderr subcommand=%s output=%s", subcommand, stderr.String()))
	}

	return stdout.Bytes(), nil
}
