/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	inputValuesBashibleCfg = "systemRegistry.internal.bashible"

	RegistryPort   = 5001
	RegistryPath   = "/system/deckhouse"
	RegistryScheme = "https"
)

var (
	RegistryHost      = fmt.Sprintf("embedded-registry.d8-system.svc:%d", RegistryPort)
	RegistryProxyHost = fmt.Sprintf("127.0.0.1:%d", RegistryPort)
)

var (
	namespaceSelector = &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{"d8-system"},
		},
	}

	masterNodeLabelSelector = &v1.LabelSelector{
		MatchLabels: map[string]string{
			"node-role.kubernetes.io/control-plane": "",
		},
	}
)
