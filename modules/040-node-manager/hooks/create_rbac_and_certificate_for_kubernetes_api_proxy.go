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
	certificatesv1 "k8s.io/api/certificates/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	kapUserName = "system:kubernetes-api-proxy"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	SANs: tls_certificate.DefaultSANs([]string{}),
	CN:   kapUserName,

	Namespace:     "kube-system", // TODO: Do we need save it here? Or we need move this tls somewhere else?
	TLSSecretName: "kubernetes-api-proxy-discovery-tls",

	FullValuesPathPrefix: "nodeManager.internal.kubernetesAPIProxyDiscovery",
	Usages: []certificatesv1.KeyUsage{
		certificatesv1.UsageClientAuth,
	},
})

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10}, // TODO: Change order to valid!
	// TODO: May be need some selectors for some cases?
}, createRBACForKubeAPIServerProxy)

func createRBACForKubeAPIServerProxy(_ context.Context, input *go_hook.HookInput) error {
	const (
		roleName        = "system:kubernetes-api-proxy-discovery"
		roleBindingName = "system:kube-apiserver-proxy-discovery"
		ns              = "default"
		userName        = kapUserName
	)

	input.PatchCollector.CreateIfNotExists(&rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"discovery.k8s.io"},
				Resources:     []string{"endpointslices"},
				ResourceNames: []string{"kubernetes"},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	})

	input.PatchCollector.CreateIfNotExists(&rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: ns,
			Labels:    map[string]string{"heritage": "deckhouse"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: userName,
			},
		},
	})

	return nil
}
