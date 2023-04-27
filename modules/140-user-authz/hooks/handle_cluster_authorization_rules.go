package hooks

import (
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

const (
	clusterAuthRuleSnapshot = "cluster_authorization_rules"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(clusterAuthRuleSnapshot),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       clusterAuthRuleSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: internal.ApplyAuthorizationRuleFilter,
		},
	},
}, internal.AuthorizationRulesHandler("userAuthz.internal.clusterAuthRuleCrds", clusterAuthRuleSnapshot))
