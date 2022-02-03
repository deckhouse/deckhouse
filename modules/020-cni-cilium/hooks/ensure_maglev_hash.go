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
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

type MaglevHash string

const (
	maglevHashKey    = "maglevHash"
	deckhouseNs      = "d8-system"
	maglevHashCmName = "d8-cilium-maglev-hash"
	hashValuePath    = "cniCilium.internal.maglevHash"
)

var (
	cmLabels = map[string]string{
		"heritage": "deckhouse",
		"module":   "cni-cilium",
	}
)

func extractMaglevHash(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to ConfigMap: %v", err)
	}

	return cm.Data[maglevHashKey], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "maglev_hash",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{deckhouseNs},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{maglevHashCmName},
			},
			FilterFunc: extractMaglevHash,
		},
	},
}, ensureMaglevHash)

func ensureMaglevHash(input *go_hook.HookInput) error {
	cms, ok := input.Snapshots["maglev_hash"]
	var hash string
	if ok && len(cms) > 0 {
		hash, ok = cms[0].(string)
		if !ok {
			return fmt.Errorf("cannot convert Kubernetes ConfigMap to MaglevHash")
		}
	}

	if len(hash) == 0 {
		hash = generateMaglevHash()

		newCM := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      maglevHashCmName,
				Namespace: deckhouseNs,
				Labels:    cmLabels,
			},
			Data: map[string]string{maglevHashKey: hash},
		}

		gvks, _, err := scheme.Scheme.ObjectKinds(newCM)
		if err != nil {
			return fmt.Errorf("missing apiVersion or kind and cannot assign it; %w", err)
		}

		for _, gvk := range gvks {
			if len(gvk.Kind) == 0 {
				continue
			}
			if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
				continue
			}
			newCM.SetGroupVersionKind(gvk)
			break
		}

		input.PatchCollector.Create(newCM, object_patch.UpdateIfExists())
	}

	input.Values.Set(hashValuePath, hash)

	return nil
}

func generateMaglevHash() string {
	rand.Seed(time.Now().Unix())

	hash := make([]byte, 12)
	rand.Read(hash)

	str := base64.StdEncoding.EncodeToString(hash)

	return str
}
