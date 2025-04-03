/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

var _ = BashibleInputHook(5, "/modules/system-registry/bashible-input")
var _ = BashibleConfigHook(6, "/modules/system-registry/bashible-config")
var _ = BashibleStatusHook(6, "/modules/system-registry/bashible-status")
