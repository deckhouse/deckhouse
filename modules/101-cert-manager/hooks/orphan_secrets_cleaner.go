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
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const certificateNameKey = "cert-manager.io/certificate-name"

type secretInfo struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Crt         []byte
}

func (m secretInfo) slugify() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	cc := secretInfo{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Annotations: secret.Annotations,
	}

	if tls, ok := secret.Data["tls.crt"]; ok {
		cc.Crt = tls
	} else if client, ok := secret.Data["client.crt"]; ok {
		cc.Crt = client
	}

	return cc, err
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
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
			FilterFunc: applySecretFilter,
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
}, cleanExpiredOrphanSecrets)

func cleanExpiredOrphanSecrets(input *go_hook.HookInput) error {
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

		// At first, we want to be sure about the cleanup safety, so we are limiting the scope to d8-.* namespaces.
		if !strings.HasPrefix(secretInfoVal.Namespace, "d8-") {
			continue
		}

		if len(secretInfoVal.Crt) > 0 {
			c, _, err := helpers.ParseOneCertificateFromPEM(secretInfoVal.Crt)
			if err != nil {
				return fmt.Errorf("can't parse certificate from the Secret %q: %v", secretSlug, err)
			}
			if len(c) == 0 {
				input.LogEntry.Infof("skipping Orphan Secret %q as it has no certificate", secretSlug)
				continue
			}
			if time.Until(c[0].NotAfter) < 0 {
				input.LogEntry.Infof("deleting expired Orphan Secret %q", secretSlug)
				input.PatchCollector.Delete("v1", "Secret", secretInfoVal.Namespace, secretInfoVal.Name)
			}
		}
	}

	return nil
}
