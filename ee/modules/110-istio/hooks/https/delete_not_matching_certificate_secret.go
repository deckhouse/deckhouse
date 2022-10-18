/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/delete_not_matching_certificate_secret"

var _ = delete_not_matching_certificate_secret.RegisterHook("istio", "d8-istio")
