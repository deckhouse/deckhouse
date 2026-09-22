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

package crdmigration

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// The list decides which CRDs get a conversion webhook stamped on them; a CAPI CRD landing in it
// would point cluster-api's own conversion at the node-controller service.
func TestConversionCRDNames(t *testing.T) {
	require.Equal(t, []string{"nodegroups.deckhouse.io", "instances.deckhouse.io"}, conversionCRDNames)
}

func TestIsConversionWebhookCurrent(t *testing.T) {
	ca := []byte("ca-bundle")

	webhookConversion := func(serviceName, namespace string, caBundle []byte) *apiextensionsv1.CustomResourceConversion {
		return &apiextensionsv1.CustomResourceConversion{
			Strategy: apiextensionsv1.WebhookConverter,
			Webhook: &apiextensionsv1.WebhookConversion{
				ClientConfig: &apiextensionsv1.WebhookClientConfig{
					Service:  &apiextensionsv1.ServiceReference{Name: serviceName, Namespace: namespace},
					CABundle: caBundle,
				},
			},
		}
	}

	tests := []struct {
		name       string
		conversion *apiextensionsv1.CustomResourceConversion
		expCurrent bool
	}{
		{name: "no conversion section", conversion: nil, expCurrent: false},
		{name: "strategy None", conversion: &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}, expCurrent: false},
		{name: "webhook without client config", conversion: &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.WebhookConverter, Webhook: &apiextensionsv1.WebhookConversion{}}, expCurrent: false},
		{name: "another service", conversion: webhookConversion("other-webhook", capiNamespace, ca), expCurrent: false},
		{name: "another namespace", conversion: webhookConversion(nodeControllerWebhookServiceName, "default", ca), expCurrent: false},
		{name: "stale CA", conversion: webhookConversion(nodeControllerWebhookServiceName, capiNamespace, []byte("old")), expCurrent: false},
		{name: "current", conversion: webhookConversion(nodeControllerWebhookServiceName, capiNamespace, ca), expCurrent: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			crd.Spec.Conversion = tc.conversion
			require.Equal(t, tc.expCurrent, isConversionWebhookCurrent(crd, ca))
		})
	}
}
