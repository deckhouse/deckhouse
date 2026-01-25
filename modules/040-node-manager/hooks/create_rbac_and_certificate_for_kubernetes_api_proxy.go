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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5}, // TODO: Change order to valid!
	// TODO: May be need some selectors for some cases?
}, dependency.WithExternalDependencies(createRBACForKubeAPIServerProxy))

func createRBACForKubeAPIServerProxy(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	const (
		roleName        = "system:kubernetes-api-proxy-discovery"
		roleBindingName = "system:kube-apiserver-proxy-discovery"
		ns              = "default"
		userName        = "system:kubernetes-api-proxy"
	)

	certExpirationSeconds := int32((time.Hour * 24 * 365 * 10).Seconds()) // 10 years

	cert, err := tls_certificate.IssueCertificate(input, dc, tls_certificate.OrderCertificateRequest{
		CommonName: userName,
		Groups: []string{
			roleName,
		},
		Usages: []certificatesv1.KeyUsage{
			certificatesv1.UsageClientAuth,
		},
		ExpirationSeconds: &certExpirationSeconds,
	})
	if err != nil {
		return errors.Wrap(err, "failed to issue certificate")
	}

	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt", cert.Certificate)
	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.key", cert.Key)

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
