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

// todo(31337Ghost) remove this hook along with removing legacy cert-manager

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const (
	legacyMetricsGroup = "legacy_orphan_secrets_metrics_hook"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("metrics_legacy"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       certsMetricSnapshot,
			ApiVersion: "certmanager.k8s.io/v1alpha1",
			Kind:       "Certificate",
			FilterFunc: applyCertMetaFilter,
		},

		{
			Name:       secretsMetricsSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applySecretMetaFilter,
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
}, legacyOrphanSecretsMetrics)

func legacyOrphanSecretsMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(legacyMetricsGroup)

	// in jq filter we see diff secret wit certs
	// so, we need iterate over certs and check key in certs map
	certSlugs := set.New()

	for _, c := range input.Snapshots[certsMetricSnapshot] {
		certSlugs.Add(c.(secretInfo).slugify())
	}

	for _, s := range input.Snapshots[secretsMetricsSnapshot] {
		secretInfoVal := s.(secretInfo)
		secretSlug := secretInfoVal.slugify()
		if certSlugs.Has(secretSlug) {
			continue
		}

		input.MetricsCollector.Set(
			"d8_orphan_secrets_without_corresponding_certificate_resources",
			1.0,
			map[string]string{
				"namespace":   secretInfoVal.Namespace,
				"secret_name": secretInfoVal.Name,
				"annotation":  "certmanager.k8s.io/certificate-name",
			},
			metrics.WithGroup(legacyMetricsGroup),
		)
	}

	return nil
}
