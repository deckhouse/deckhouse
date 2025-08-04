/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

type Inputs = State

type State struct {
	HTTP string `json:"http,omitempty"`
}

func (state *State) Process() error {
	if strings.TrimSpace(state.HTTP) == "" {
		if randomValue, err := pki.GenerateRandomSecret(); err == nil {
			state.HTTP = randomValue
		} else {
			return fmt.Errorf("cannot generate HTTP secret: %w", err)
		}
	}

	return nil
}
