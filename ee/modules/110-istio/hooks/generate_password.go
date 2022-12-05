/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/go_lib/hooks/generate_password"
)

const (
	moduleValuesKey = "istio"
	authSecretNS    = "d8-istio"
	authSecretName  = "kiali-basic-auth"
)

var _ = generate_password.RegisterHook(moduleValuesKey, authSecretNS, authSecretName)
