/*
Copyright 2021 Flant CJSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*
Description:
	get etcd certificates from secret and store it into internal values
*/

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue + "/etcd-certs",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "etcd-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-pki"}},
			FilterFunc:   syncEtcdFilter,
		},
	},
}, handleSyncEtcdCerts)

func syncEtcdFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(unstructured, &sec)
	if err != nil {
		return nil, err
	}

	var ec etcdCerts

	if ca, ok := sec.Data["etcd-ca.crt"]; ok {
		ec.CA = ca
		ec.Cert = ca
	}

	if key, ok := sec.Data["etcd-ca.key"]; ok {
		ec.Key = key
	}

	return ec, nil
}

type etcdCerts struct {
	CA   []byte
	Cert []byte
	Key  []byte
}

func handleSyncEtcdCerts(input *go_hook.HookInput) error {
	snap := input.Snapshots["etcd-secret"]

	if len(snap) == 0 {
		return nil
	}

	cert := snap[0].(etcdCerts)

	if len(cert.CA) > 0 {
		input.Values.Set("controlPlaneManager.internal.etcdCerts.ca", cert.CA)
	}
	if len(cert.Cert) > 0 {
		input.Values.Set("controlPlaneManager.internal.etcdCerts.crt", cert.Cert)
	}
	if len(cert.Key) > 0 {
		input.Values.Set("controlPlaneManager.internal.etcdCerts.key", cert.Key)
	}

	return nil
}
