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
	"crypto/x509"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RemoteWrite struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

func filterRemoteWriteCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from PrometheusRemoteWrite: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("prometheusRemoteWrite has no spec field")
	}
	ca, ok, err := unstructured.NestedString(obj.Object, "spec", "tlsConfig", "ca")
	if err == nil && ok {
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM([]byte(ca))
		if !ok {
			return new(RemoteWrite), fmt.Errorf("tlsConfig:ca in PremetheusRemoteWrite not valid: %v", err)
		}
	}
	rw := new(RemoteWrite)
	rw.Name = obj.GetName()
	rw.Spec = spec

	return rw, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/remote_write",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "prometheusremotewrite",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "PrometheusRemoteWrite",
			FilterFunc: filterRemoteWriteCRD,
		},
	},
}, remoteWriteHandler)

func remoteWriteHandler(input *go_hook.HookInput) error {
	prw := input.Snapshots["prometheusremotewrite"]

	if len(prw) == 0 {
		input.Values.Set("prometheus.internal.remoteWrite", make([]interface{}, 0))
		return nil
	}

	input.Values.Set("prometheus.internal.remoteWrite", prw)

	return nil
}
