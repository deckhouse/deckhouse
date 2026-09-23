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
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	admissionWebhookCertSnapshotName = "admissionWebhookCert"
	admissionWebhookCertSecretName   = "admission-webhook-certs"
	admissionWebhookCertNamespace    = "d8-multitenancy-manager"
	certCAValuesPath                 = "multitenancyManager.internal.admissionWebhookCert.ca"
)

var errWebhookCertNotIssued = errors.New("webhook certificate is not issued yet")

type admissionWebhookCertSnap struct {
	CA string `json:"ca"`
}

// admissionWebhookCertWatch re-runs grant webhook hooks when Helm writes a new
// admission-webhook-certs. RegisterInternalTLSHook already watches the same
// Secret (snapshot "secret") to refresh values; these two extra watches are
// the grant hooks' own informers so a rotation wakes them without a Grantable
// CR change.
func admissionWebhookCertWatch() go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       admissionWebhookCertSnapshotName,
		ApiVersion: "v1",
		Kind:       "Secret",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{admissionWebhookCertNamespace},
			},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{admissionWebhookCertSecretName},
		},
		FilterFunc: filterAdmissionWebhookCert,
	}
}

func filterAdmissionWebhookCert(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	return admissionWebhookCertSnap{CA: string(secret.Data["ca.crt"])}, nil
}

// admissionWebhookCABundle prefers ca.crt from the Secret the pod mounts and
// serves. The TLS hook writes the new cert into values first (OnBeforeHelm);
// Helm then renders the Secret. Values therefore lead, the Secret lags. We
// still follow the Secret: that is what the webhook process presents, so
// caBundle must match it, not the newer values. An empty result means the
// cert is not issued yet — callers must fail only on create, not on update.
func admissionWebhookCABundle(input *go_hook.HookInput) (string, error) {
	snaps, err := sdkobjectpatch.UnmarshalToStruct[admissionWebhookCertSnap](input.Snapshots, admissionWebhookCertSnapshotName)
	if err != nil {
		return "", fmt.Errorf("unmarshal admission webhook cert snapshot: %w", err)
	}
	if len(snaps) > 0 && snaps[0].CA != "" {
		return snaps[0].CA, nil
	}
	return input.Values.Get(certCAValuesPath).String(), nil
}
