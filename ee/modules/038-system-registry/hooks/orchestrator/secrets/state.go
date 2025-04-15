/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package secrets

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
)

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
