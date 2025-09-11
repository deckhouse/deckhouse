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
	"fmt"
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/etcd"
)

const (
	moduleQueue = "/modules/control-plane-manager"
)

type etcdInstance struct {
	Endpoint  string
	MaxDbSize int64
	PodName   string
	Node      string
}

func getETCDClient(input *go_hook.HookInput, dc dependency.Container, endpoints []string) (etcd.Client, error) {
	certs, err := sdkobjectpatch.UnmarshalToStruct[certificate.Certificate](input.Snapshots, "etcd-certificate")
	if err != nil {
		return nil, fmt.Errorf("unmarshal etcd-certificate: %w", err)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("etcd credentials not found")
	}

	cert := certs[0]

	if cert.CA == "" || cert.Cert == "" || cert.Key == "" {
		return nil, fmt.Errorf("etcd credentials not found")
	}

	caCert, clientCert, err := certificate.ParseCertificatesFromPEM(cert.CA, cert.Cert, cert.Key)
	if err != nil {
		return nil, err
	}

	return dc.GetEtcdClient(endpoints, etcd.WithClientCert(clientCert, caCert), etcd.WithInsecureSkipVerify())
}

var (
	etcdSecretK8sConfig = go_hook.KubernetesConfig{
		Name:       "etcd-certificate",
		ApiVersion: "v1",
		Kind:       "Secret",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kube-system"},
			},
		},
		NameSelector:                 &types.NameSelector{MatchNames: []string{"d8-pki"}},
		ExecuteHookOnSynchronization: ptr.To(false),
		ExecuteHookOnEvents:          ptr.To(false),
		FilterFunc:                   syncEtcdFilter,
	}
)

func syncEtcdFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(unstructured, &sec)
	if err != nil {
		return nil, err
	}

	var cert certificate.Certificate

	if ca, ok := sec.Data["etcd-ca.crt"]; ok {
		cert.CA = string(ca)
		cert.Cert = string(ca)
	}

	if key, ok := sec.Data["etcd-ca.key"]; ok {
		cert.Key = string(key)
	}

	return cert, nil
}

func gb(n int64) int64 {
	return n * 1024 * 1024 * 1024
}

func gbFloat(n float64) int64 {
	return int64(math.Floor(n * 1024 * 1024 * 1024))
}
