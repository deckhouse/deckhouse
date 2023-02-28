package internal

import "github.com/flant/shell-operator/pkg/kube_events_manager/types"

var Namespace = "d8-ingress-nginx"

func NsSelector() *types.NamespaceSelector {
	return &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{Namespace},
		},
	}
}
