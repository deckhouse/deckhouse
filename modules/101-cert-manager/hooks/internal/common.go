package internal

import (
	"fmt"

	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

const Namespace = "d8-cert-manager"

func Queue(name string) string {
	return fmt.Sprintf("/modules/cert-manager/%s", name)
}

func NsSelector() *types.NamespaceSelector {
	return &types.NamespaceSelector{
		NameSelector: &types.NameSelector{
			MatchNames: []string{Namespace},
		},
	}
}
