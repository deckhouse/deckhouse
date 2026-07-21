// Copyright 2024 Flant JSC
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

package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type Service struct {
	pb.UnimplementedValidationServer

	schemaStore   *config.SchemaStore
	globalOptions *options.GlobalOptions
}

func New(schemaStore *config.SchemaStore, globalOptions *options.GlobalOptions) *Service {
	return &Service{
		schemaStore:   schemaStore,
		globalOptions: globalOptions,
	}
}

// ensureProviderBundle lazily delivers the external provider bundle so
// validation works on a cold pod.
func (s *Service) ensureProviderBundle(ctx context.Context, provider, configYAML string) error {
	docs := input.YAMLSplitRegexp.Split(strings.TrimSpace(configYAML), -1)
	return config.EnsureProviderBundle(ctx, provider, docs, s.globalOptions)
}

func optionsFromRequest(opts *pb.ValidateOptions) []config.ValidateOption {
	return []config.ValidateOption{
		config.ValidateOptionCommanderMode(opts.CommanderMode),
		config.ValidateOptionStrictUnmarshal(opts.StrictUnmarshal),
		config.ValidateOptionValidateExtensions(opts.ValidateExtensions),
		config.ValidateOptionRequiredSSHHost(opts.RequiredSshHost),
	}
}

//nolint:musttag
func errorToResponse(err error) (string, error) {
	if err == nil {
		return "", nil
	}

	var e *config.ValidationError
	if errors.As(err, &e) {
		validateErrBytes, marshalErr := json.Marshal(e)
		if marshalErr != nil {
			return "", fmt.Errorf("marshalling validation error %w: %w", err, marshalErr)
		}
		return string(validateErrBytes), nil
	}

	e = &config.ValidationError{Errors: []config.Error{
		{
			Messages: []string{err.Error()},
		},
	}}
	validateErrBytes, marshalErr := json.Marshal(e)
	if marshalErr != nil {
		return "", fmt.Errorf("marshalling validation error %w: %w", err, marshalErr)
	}
	return string(validateErrBytes), nil
}
