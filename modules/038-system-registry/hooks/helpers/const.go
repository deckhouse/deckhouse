/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

var (
	NamespaceSelector = &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{"d8-system"},
		},
	}

	RegistryServiceDNSName = fmt.Sprintf("%s.%s.svc", RegistryServiceName, RegistryServiceNamespace)
)

const (
	RegistryServiceName      = "registry"
	RegistryServiceNamespace = "d8-system"
)
