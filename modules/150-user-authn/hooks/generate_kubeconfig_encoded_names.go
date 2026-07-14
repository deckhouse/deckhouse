/*
Copyright 2021 Flant JSC

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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, kubeconfigNamesHandler)

// kubeconfigPublishAPIClientID is the client_id of the OAuth2Client that
// covers the publishAPI kubeconfig entry. Mirrored verbatim in
// `templates/dex/kubeconfig-oauth2clients.yaml`,
// `templates/dex/kubernetes-dex-client-configuration.yaml` and the trustedPeers
// list in `templates/dex/oauth2client.yaml`. Keep them in sync.
const kubeconfigPublishAPIClientID = "kubeconfig-publish-api"

// slugifyKubeconfigID mirrors the slug transform applied to client_id in
// `templates/dex/kubernetes-dex-client-configuration.yaml`,
// `templates/dex/kubeconfig-oauth2clients.yaml` and the trustedPeers list in
// `templates/dex/oauth2client.yaml`. Keep them in sync.
func slugifyKubeconfigID(id string) string {
	s := strings.ReplaceAll(id, "@", "-at-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

func kubeconfigNamesHandler(_ context.Context, input *go_hook.HookInput) error {
	const (
		kubeconfigsPath             = "userAuthn.kubeconfigGenerator"
		encodedNamesPath            = "userAuthn.internal.kubeconfigEncodedNames"
		clientEncodedNamesPath      = "userAuthn.internal.kubeconfigClientEncodedNames"
		publishAPIEncodedNamePath   = "userAuthn.internal.kubeconfigPublishAPIEncodedName"
		publishAPIEnabledPath       = "userAuthn.internal.publishAPI.enabled"
		publishAPIAddKubeconfigPath = "userAuthn.internal.publishAPI.addKubeconfigGeneratorEntry"
	)

	kubeconfigs := input.ConfigValues.Get(kubeconfigsPath).Array()

	legacyEncodedNames := make([]string, 0, len(kubeconfigs))
	clientEncodedNames := make([]string, 0, len(kubeconfigs))
	clientIDs := make([]string, 0, len(kubeconfigs))

	for i, entry := range kubeconfigs {
		legacy := fmt.Sprintf("kubeconfig-generator-%d", i)
		legacyEncodedNames = append(legacyEncodedNames, encoding.ToFnvLikeDex(legacy))

		id := entry.Get("id").String()
		clientID := fmt.Sprintf("kubeconfig-%s", slugifyKubeconfigID(id))
		clientIDs = append(clientIDs, clientID)
		clientEncodedNames = append(clientEncodedNames, encoding.ToFnvLikeDex(clientID))
	}

	if err := validateKubeconfigClientIDs(kubeconfigs, clientIDs); err != nil {
		return err
	}

	if !input.ConfigValues.Exists(kubeconfigsPath) {
		input.Values.Remove(encodedNamesPath)
		input.Values.Remove(clientEncodedNamesPath)
	} else {
		input.Values.Set(encodedNamesPath, legacyEncodedNames)
		input.Values.Set(clientEncodedNamesPath, clientEncodedNames)
	}

	if input.Values.Get(publishAPIEnabledPath).Bool() && input.Values.Get(publishAPIAddKubeconfigPath).Bool() {
		input.Values.Set(publishAPIEncodedNamePath, encoding.ToFnvLikeDex(kubeconfigPublishAPIClientID))
	} else {
		input.Values.Remove(publishAPIEncodedNamePath)
	}

	return nil
}

// validateKubeconfigClientIDs catches client_id collisions caused by the
// id→slug transform:
//
//   - C1: a slug-derived client_id collides with the legacy
//     `kubeconfig-generator-N` for any N in the current entry range, or with
//     the reserved publishAPI client_id.
//   - C2: two entries slugify to the same client_id.
func validateKubeconfigClientIDs(kubeconfigs []gjson.Result, clientIDs []string) error {
	legacyReserved := make(map[string]struct{}, len(clientIDs)+1)
	for i := range clientIDs {
		legacyReserved[fmt.Sprintf("kubeconfig-generator-%d", i)] = struct{}{}
	}
	legacyReserved[kubeconfigPublishAPIClientID] = struct{}{}

	seen := make(map[string]int, len(clientIDs))
	for i, clientID := range clientIDs {
		if _, ok := legacyReserved[clientID]; ok {
			return fmt.Errorf(
				"userAuthn.kubeconfigGenerator[%d].id=%q produces reserved client_id %q "+
					"(reserved: kubeconfig-generator-N for the current entry range, and %q)",
				i, kubeconfigs[i].Get("id").String(), clientID, kubeconfigPublishAPIClientID)
		}
		if prev, ok := seen[clientID]; ok {
			return fmt.Errorf(
				"userAuthn.kubeconfigGenerator[%d].id=%q and [%d].id=%q slugify to the same client_id %q; "+
					"ids must produce distinct client_ids after replacing '@'→'-at-' and ':'→'-'",
				prev, kubeconfigs[prev].Get("id").String(),
				i, kubeconfigs[i].Get("id").String(),
				clientID)
		}
		seen[clientID] = i
	}
	return nil
}
