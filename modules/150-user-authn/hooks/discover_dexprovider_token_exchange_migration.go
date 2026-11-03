/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// dexProviderTokenExchangeMigrationName is the ConfigMap the user-authn controller creates once it has set
// spec.tokenExchange.enabled on the DexProviders that existed before the field
// (images/user-authn-controller/src/internal/migration/README.md).
const dexProviderTokenExchangeMigrationName = "dexprovider-token-exchange-migration"

// Until the ConfigMap exists, a DexProvider without spec.tokenExchange keeps exchanging tokens, as it did before the
// field existed: the Dex config is rendered on the upgrade before the controller has run the migration, and pipelines
// would get 400 in between. After it, a DexProvider without the field does not exchange tokens.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "migration",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"d8-user-authn"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{dexProviderTokenExchangeMigrationName}},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return obj.GetName(), nil
			},
		},
	},
}, discoverDexProviderTokenExchangeMigration)

func discoverDexProviderTokenExchangeMigration(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set("userAuthn.internal.dexProviderTokenExchangeMigrated", len(input.Snapshots.Get("migration")) > 0)
	return nil
}
