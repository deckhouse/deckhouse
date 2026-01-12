// Copyright 2025 Flant JSC
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

package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var ErrNotFound = errors.New("not found")

func UnmarshalToFromMessage[T any](msg json.RawMessage) (*T, error) {
	var result T

	if err := json.Unmarshal(msg, &result); err != nil {
		return nil, fmt.Errorf(
			"Failed to unmarshal to %s: %w",
			reflect.TypeFor[T]().String(),
			err,
		)
	}

	return &result, nil
}

func UnmarshalToFromMessageMap[T any](msg map[string]json.RawMessage, key string) (*T, error) {
	rawMsg, ok := msg[key]
	if !ok {
		return nil, fmt.Errorf("Key %s %w", key, ErrNotFound)
	}

	result, err := UnmarshalToFromMessage[T](rawMsg)
	if err != nil {
		return nil, fmt.Errorf("%w from key %s", err, key)
	}

	return result, nil
}
