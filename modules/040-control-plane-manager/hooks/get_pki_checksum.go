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
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*
Description:
	This hook founds Secret: d8-pki in NS: kube-system, gets all data from that secret
	sort it by keys and calculate sha256 checksum, which stores in values: controlPlaneManager.internal.pkiChecksum
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pki_checksum",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-pki"},
			},
			FilterFunc: filterPkiSecret,
		},
	},
}, handlePKIChecksum)

type secretData map[string][]byte

func filterPkiSecret(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(unstructured, &sec)
	if err != nil {
		return nil, err
	}

	return secretData(sec.Data), nil
}

func handlePKIChecksum(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("pki_checksum")
	if len(snaps) == 0 {
		return fmt.Errorf(`there is no Secret named "d8-pki" in NS "kube-system"`)
	}

	var sData secretData
	err := snaps[0].UnmarshalTo(&sData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'pki_checksum' snapshot: %w", err)
	}

	keys := make([]string, 0, len(sData))

	// sort map values by key
	for k := range sData {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// fill buffer with sorted key/values
	hash := sha256.New()
	for _, k := range keys {
		hash.Write([]byte(k))
		hash.Write(sData[k])
	}
	sha256sum := fmt.Sprintf("%x", hash.Sum(nil))

	input.Values.Set("controlPlaneManager.internal.pkiChecksum", sha256sum)

	return nil
}
