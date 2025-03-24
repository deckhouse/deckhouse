/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pki

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

const (
	inputValuesPrefix = "systemRegistry.internal.pki"
)

var (
	inputValuesMode = fmt.Sprintf("%s.mode", inputValuesPrefix)

	inputValuesCA    = fmt.Sprintf("%s.ca", inputValuesPrefix)
	inputValuesToken = fmt.Sprintf("%s.token", inputValuesPrefix)
	inputValuesProxy = fmt.Sprintf("%s.proxy", inputValuesPrefix)
)

var (
	namespaceSelector = &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{"d8-system"},
		},
	}
)
