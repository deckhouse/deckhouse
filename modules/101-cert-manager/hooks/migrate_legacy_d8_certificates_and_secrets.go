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

// Legacy deckhouse certificates could not be removed by Helm
// remove them manually.

// todo(31337Ghost) This hook should be deleted when legacy cert-manager removed

package hooks

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

type legacySecretInfo struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Labels      map[string]string
}

func (m legacySecretInfo) slugify() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}

func applyLegacySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return legacySecretInfo{
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		Annotations: obj.GetAnnotations(),
		Labels:      obj.GetLabels(),
	}, nil
}

type secretInfo struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Crt         []byte
}

func (m secretInfo) slugify() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}

func applyCertMetaFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// why we parse unstructured manually here?
	// cert-manager 0.10.1 replace client go for old version
	// k8s.io/client-go v0.19.11 => v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	// and store cert-manager files here will confuse
	un := obj.UnstructuredContent()
	specRaw, ok := un["spec"]
	if !ok {
		return nil, fmt.Errorf("cannot spec for certificate")
	}

	spec, ok := specRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot cast spec for certificate")
	}

	secretNameRaw, ok := spec["secretName"]
	if !ok {
		return nil, fmt.Errorf("cannot get spec.SecretName for certificate")
	}

	secretName, ok := secretNameRaw.(string)
	if !ok {
		return nil, fmt.Errorf("cannot cast spec.SecretName for certificate")
	}

	return secretInfo{
		Namespace: obj.GetNamespace(),
		Name:      secretName,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("certificates"),
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 10,
	},
	OnAfterHelm: &go_hook.OrderedConfig{
		Order: 10,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "certificates_legacy",
			ApiVersion: "certmanager.k8s.io/v1alpha1",
			Kind:       "Certificate",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
					{
						Key:      "app.kubernetes.io/managed-by",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"Helm"},
					},
				},
			},
			FilterFunc: applyLegacyCertManagerCRFilter,
		},
		{
			Name:       "certificates",
			ApiVersion: "cert-manager.io/v1",
			Kind:       "Certificate",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
					{
						Key:      "app.kubernetes.io/managed-by",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"Helm"},
					},
				},
			},
			FilterFunc: applyCertMetaFilter,
		},
		{
			Name:       "secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyLegacySecretFilter,
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "type",
						Operator: "Equals",
						Value:    "kubernetes.io/tls",
					},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "certmanager.k8s.io/certificate-name",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		},
	},
}, removeLegacyCerts)

func removeLegacyCerts(input *go_hook.HookInput) error {
	snapCertsLegacy := input.Snapshots["certificates_legacy"]
	for _, sn := range snapCertsLegacy {
		cert := sn.(legacyObject)

		input.PatchCollector.Delete("certmanager.k8s.io/v1alpha1", "Certificate", cert.Namespace, cert.Name)
	}

	// Before removing annotations from Secrets all old Certificates should be deleted.
	if len(snapCertsLegacy) != 0 {
		return nil
	}

	certsNewSecretSlugs := set.New()
	for _, sn := range input.Snapshots["certificates"] {
		certsNewSecretSlugs.Add(sn.(secretInfo).slugify())
	}

	snapSecrets := input.Snapshots["secrets"]
	for _, sn := range snapSecrets {
		secret := sn.(legacySecretInfo)

		// We are migrating Secrets only in d8-.* namespaces.
		if !strings.HasPrefix(secret.Namespace, "d8-") {
			continue
		}

		// We are migrating only Secrets that has referenced Certificate:
		// - of the new apiVersion;
		// - having label heritage=deckhouse;
		//
		// That covers the case when user can have custom Ingress with legacy Certificate for our components.
		secretSlug := secret.slugify()
		if !certsNewSecretSlugs.Has(secretSlug) {
			continue
		}

		annotationsToRemove := make(map[string]interface{})
		for key := range secret.Annotations {
			if strings.HasPrefix(key, "certmanager.k8s.io/") {
				annotationsToRemove[key] = nil
			}
		}

		metadata := make(map[string]interface{})
		metadata["labels"] = map[string]interface{}{"certmanager.k8s.io/certificate-name": nil}
		if len(annotationsToRemove) > 0 {
			metadata["annotations"] = annotationsToRemove
		}
		annotationsPatch := map[string]interface{}{"metadata": metadata}

		input.PatchCollector.MergePatch(annotationsPatch, "v1", "Secret", secret.Namespace, secret.Name)
	}

	return nil
}
