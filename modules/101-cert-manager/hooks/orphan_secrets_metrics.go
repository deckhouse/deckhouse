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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const (
	certsMetricSnapshot    = "certificates"
	secretsMetricsSnapshot = "secrets"

	metricsGroup = "orphan_secrets_metrics_hook"
)

func applySecretMetaFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return secretInfo{
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		Annotations: obj.GetAnnotations(),
	}, nil
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
	Queue: internal.Queue("metrics"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       certsMetricSnapshot,
			ApiVersion: "cert-manager.io/v1",
			Kind:       "Certificate",
			FilterFunc: applyCertMetaFilter,
		},

		{
			Name:       secretsMetricsSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applySecretMetaFilter,
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "type",
						Operator: "Equals",
						Value:    "kubernetes.io/tls",
					},
				},
			},
		},
	},
}, orphanSecretsMetrics)

func orphanSecretsMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricsGroup)

	// in jq filter we see diff secret wit certs
	// so, we need iterate over certs and check key in certs map
	certSlugs := set.New()

	for _, c := range input.Snapshots[certsMetricSnapshot] {
		certSlugs.Add(c.(secretInfo).slugify())
	}

	for _, s := range input.Snapshots[secretsMetricsSnapshot] {
		secretInfoVal := s.(secretInfo)

		// Skip Secrets that are not related to cert-manager.
		_, hasLbl := secretInfoVal.Annotations[certificateNameKey]
		if !hasLbl {
			continue
		}

		secretSlug := secretInfoVal.slugify()
		if certSlugs.Has(secretSlug) {
			continue
		}

		// Skip metric for Orphan Secrets in d8-.* namespaces to mute alerts on them.
		// Those Secrets be automatically deleted after expire by `orphan_secrets_cleaner.go`.
		if strings.HasPrefix(secretInfoVal.Namespace, "d8-") {
			continue
		}

		input.MetricsCollector.Set(
			"d8_orphan_secrets_without_corresponding_certificate_resources",
			1.0,
			map[string]string{
				"namespace":   secretInfoVal.Namespace,
				"secret_name": secretInfoVal.Name,
				"annotation":  "cert-manager.io/certificate-name",
			},
			metrics.WithGroup(metricsGroup),
		)
	}

	return nil
}
