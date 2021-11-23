/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, generatePassword)

const defaultLocationTemplate = `
[ {
  "users": {"admin": "%s"},
  "location": "/"
} ]
`

func generatePassword(input *go_hook.HookInput) error {
	_, ok := input.Values.GetOk("basicAuth.locations")
	if ok {
		return nil
	}

	rawLocations := make([]map[string]interface{}, 0)

	locations := fmt.Sprintf(defaultLocationTemplate, pwgen.AlphaNum(20))
	err := json.Unmarshal([]byte(locations), &rawLocations)
	if err != nil {
		return err
	}

	input.ConfigValues.Set("basicAuth.locations", rawLocations)
	return nil
}
