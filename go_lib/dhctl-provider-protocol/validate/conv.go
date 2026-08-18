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

package validate

import (
	"encoding/json"
	"fmt"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
)

// ToPBRequest encodes the input for the wire.
func ToPBRequest(input Input) (*protogen.ValidateRequest, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode validate input: %w", err)
	}
	return &protogen.ValidateRequest{InputJson: inputJSON}, nil
}

// FromPBRequest decodes the input a request carries.
func FromPBRequest(req *protogen.ValidateRequest) (Input, error) {
	var input Input
	if err := json.Unmarshal(req.GetInputJson(), &input); err != nil {
		return input, fmt.Errorf("decode validate input: %w", err)
	}
	return input, nil
}

// ToPBResponse encodes a result for the wire.
func ToPBResponse(result Result) *protogen.ValidateResponse {
	return &protogen.ValidateResponse{
		Errors:   ToPBViolations(result.Errors()),
		Warnings: ToPBViolations(result.Warnings()),
	}
}

// FromPBResponse decodes a response back into a Result, so a host renders its
// user-facing text with the same code the plugin would have used.
func FromPBResponse(resp *protogen.ValidateResponse) Result {
	if resp == nil {
		return Result{}
	}
	return NewResult(
		FromPBViolations(resp.GetErrors()),
		FromPBViolations(resp.GetWarnings()),
	)
}

// ToPBViolations renders violations for the wire. Value becomes a string via
// fmt.Sprint: it is a display-only field, and a plugin that must not disclose a
// value sends a placeholder instead of the value itself.
func ToPBViolations(violations []Violation) []*protogen.Violation {
	if len(violations) == 0 {
		return nil
	}

	ret := make([]*protogen.Violation, 0, len(violations))
	for _, violation := range violations {
		wire := &protogen.Violation{
			Path:    violation.Path,
			Code:    violation.Code,
			Message: violation.Message,
		}
		if violation.Value != nil {
			wire.Value = fmt.Sprint(violation.Value)
		}
		ret = append(ret, wire)
	}
	return ret
}

// FromPBViolations decodes violations off the wire.
func FromPBViolations(violations []*protogen.Violation) []Violation {
	if len(violations) == 0 {
		return nil
	}

	ret := make([]Violation, 0, len(violations))
	for _, violation := range violations {
		ret = append(ret, Violation{
			Path:    violation.GetPath(),
			Code:    violation.GetCode(),
			Message: violation.GetMessage(),
			Value:   valueOrNil(violation.GetValue()),
		})
	}
	return ret
}

func valueOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
