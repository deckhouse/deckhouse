/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package submodule

import (
	"errors"
)

const (
	submodulesValuesPrefix = "systemRegistry.internal"
)

var (
	ErrNotFound = errors.New("value not found")
)
