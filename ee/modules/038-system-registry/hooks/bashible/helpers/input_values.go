/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

var (
	ErrInputValueNotExist = errors.New("input value not exist")
)

func UnmarshalInputValue(input *go_hook.HookInput, objLocation string, objOut any) error {
	obj := input.Values.Get(objLocation)

	if !obj.Exists() {
		return fmt.Errorf("failed to get \"%s\": %w", objLocation, ErrInputValueNotExist)
	}

	if err := json.Unmarshal([]byte(obj.Raw), objOut); err != nil {
		return err
	}
	return nil
}
