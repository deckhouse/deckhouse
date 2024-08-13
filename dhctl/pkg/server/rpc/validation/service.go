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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

type Service struct {
	pb.UnimplementedValidationServer

	schemaStore *config.SchemaStore
}

func New(schemaStore *config.SchemaStore) *Service {
	return &Service{
		schemaStore: schemaStore,
	}
}

func optionsFromRequest(opts *pb.ValidateOptions) []config.ValidateOption {
	return []config.ValidateOption{
		config.ValidateOptionCommanderMode(opts.CommanderMode),
		config.ValidateOptionStrictUnmarshal(opts.StrictUnmarshal),
		config.ValidateOptionValidateExtensions(opts.ValidateExtensions),
		config.ValidateOptionRequiredSSHHost(opts.RequiredSshHost),
	}
}

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
