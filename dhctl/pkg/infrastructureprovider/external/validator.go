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
	"errors"
	"fmt"
	"strings"
	"time"

	otattribute "go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/client"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

const requestTimeout = 10 * time.Second

func Validate(ctx context.Context, binaryPath string, input config.ProviderInput) error {
	ctx, span := telemetry.StartSpan(ctx, "external.validate")
	defer span.End()

	span.SetAttributes(
		otattribute.String("provider.name", input.ProviderName),
		otattribute.String("provider.binary", binaryPath),
	)

	wireInput, err := toWireInput(input)
	if err != nil {
		return fmt.Errorf("build validate request: %w", err)
	}

	resp, err := validate(ctx, binaryPath, wireInput)
	if err != nil {
		return fmt.Errorf("run provider %q validator: %w", input.ProviderName, err)
	}

	if warningsStr := violationsToStrings(resp.GetWarnings()); len(warningsStr) > 0 {
		reportWarnings(ctx, input.ProviderName, warningsStr)
	}

	if errorsStr := violationsToStrings(resp.GetErrors()); len(errorsStr) > 0 {
		return fmt.Errorf("provider %q validation failed: %s", input.ProviderName, strings.Join(errorsStr, "\n"))
	}
	return nil
}

// validate spawns one validator, asks it, and tears it back down.
func validate(ctx context.Context, binaryPath string, input validatev1.Input) (_ *validatev1.ValidateResponse, retErr error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	validator, err := newListeningValidator(ctx, binaryPath)
	if err != nil {
		return nil, fmt.Errorf("new validator process %q: %w", binaryPath, err)
	}

	// The returned context is the validator's own life: a request made with it fails
	// the moment the process dies, instead of waiting out the call deadline.
	ctx, err = validator.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("start validator process %q: %w", binaryPath, err)
	}

	defer func() {
		if err := validator.Stop(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("stop validator process %q: %w", binaryPath, err))
		}
	}()

	reqCtx, reqCancel := context.WithTimeout(ctx, requestTimeout)
	defer reqCancel()

	resp, err := requestValidate(reqCtx, validator.Endpoint(), input)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			err = errors.Join(err, cause)
		}

		return nil, fmt.Errorf("call validator on %s: %w", validator.Endpoint(), err)
	}

	return resp, nil
}

func requestValidate(ctx context.Context, ep endpoint, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	conn, err := grpc.NewClient(
		ep.DialTarget(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return nil, err
	}

	defer func() { _ = conn.Close() }()

	return client.
		NewValidateClient(conn, client.NewConfig()).
		Validate(ctx, input)
}

func reportWarnings(ctx context.Context, providerName string, warnings []string) {
	logger := dhlog.FromContext(ctx)
	logger.WarnContext(ctx, "=================================================================")
	logger.WarnContext(ctx, fmt.Sprintf("WARNING: %q provider validation.", providerName))
	for _, warn := range warnings {
		logger.WarnContext(ctx, fmt.Sprintf(" - %s", warn))
	}
	logger.WarnContext(ctx, "=================================================================")
}
