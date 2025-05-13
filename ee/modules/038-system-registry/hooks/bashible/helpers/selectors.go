/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helpers

import (
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	NamespaceSelector = &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{"d8-system"},
		},
	}

	MasterNodeLabelSelector = &v1.LabelSelector{
		MatchLabels: map[string]string{
			"node-role.kubernetes.io/control-plane": "",
		},
	}
)
