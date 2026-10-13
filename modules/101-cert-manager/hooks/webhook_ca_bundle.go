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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

// When the webhook CA changes, the pods of the webhook switch to the new certificate one by one
// during a rollout. The API server must trust both CAs until the last pod with the old certificate
// is gone, otherwise every request that lands on a pod with the "wrong" certificate fails with
// "x509: certificate signed by unknown authority" and the API server returns HTTP 500 for every
// create and update of a cert-manager resource.
//
// The hook keeps the CA that the webhook configurations trust right now in the bundle next to the
// current CA until the webhook Deployment has fully rolled out the pod template that carries the
// current CA and every pod of an older template is gone. The template marks that pod template with
// the webhookCAChecksumAnnotation.
//
// The Deployment status alone is not enough: status.replicas does not count terminating pods, while a
// terminating pod still serves its certificate during the preStop delay.

const (
	webhookCAChecksumAnnotation = "checksum/ca"

	caBundleValuesPath         = "certManager.internal.webhookCert.caBundle"
	caRotationActiveValuesPath = "certManager.internal.webhookCert.caRotationInProgress"
	webhookCAValuesPath        = "certManager.internal.webhookCert.ca"

	webhookConfigurationSnapshot = "webhook_configuration"
	webhookDeploymentSnapshot    = "webhook_deployment"
	webhookPodsSnapshot          = "webhook_pods"
)

type webhookDeploymentState struct {
	CAChecksum         string
	Generation         int64
	ObservedGeneration int64
	Replicas           int32
	UpdatedReplicas    int32
}

// rolledOutWith reports whether no pod of an older template is left, that is, no pod can still serve
// a certificate of the previous CA. Availability does not matter: a pod of the new template that is not
// ready yet serves nothing, and waiting for it would keep the previous CA trusted for as long as a
// master stays NotReady.
func (s webhookDeploymentState) rolledOutWith(caChecksum string) bool {
	return s.CAChecksum == caChecksum &&
		s.ObservedGeneration >= s.Generation &&
		s.Replicas == s.UpdatedReplicas
}

func applyWebhookConfigurationCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	webhooks, _, err := unstructured.NestedSlice(obj.Object, "webhooks")
	if err != nil {
		return nil, fmt.Errorf("cannot get webhooks of %s: %w", obj.GetName(), err)
	}
	if len(webhooks) == 0 {
		return "", nil
	}

	first, ok := webhooks[0].(map[string]interface{})
	if !ok {
		return "", nil
	}
	caBundle, _, err := unstructured.NestedString(first, "clientConfig", "caBundle")
	if err != nil {
		return nil, fmt.Errorf("cannot get caBundle of %s: %w", obj.GetName(), err)
	}

	// The unstructured object keeps caBundle base64-encoded, as it is stored in the API.
	return caBundle, nil
}

func applyWebhookDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var d appsv1.Deployment
	if err := sdk.FromUnstructured(obj, &d); err != nil {
		return nil, err
	}

	return webhookDeploymentState{
		CAChecksum:         d.Spec.Template.Annotations[webhookCAChecksumAnnotation],
		Generation:         d.Generation,
		ObservedGeneration: d.Status.ObservedGeneration,
		Replicas:           d.Status.Replicas,
		UpdatedReplicas:    d.Status.UpdatedReplicas,
	}, nil
}

type webhookPodState struct {
	CAChecksum string
	// Finished is set for a pod that has completed or failed. Such a pod serves nothing, but it stays
	// after a node shutdown or an eviction until the pod garbage collector removes it, which may take
	// indefinitely long.
	Finished bool
}

// applyWebhookPodCAFilter returns the CA checksum of the template a pod was created from. Terminating
// pods stay in the snapshot until they are deleted.
func applyWebhookPodCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	phase, _, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err != nil {
		return nil, fmt.Errorf("cannot get phase of %s: %w", obj.GetName(), err)
	}
	return webhookPodState{
		CAChecksum: obj.GetAnnotations()[webhookCAChecksumAnnotation],
		Finished:   phase == string(corev1.PodSucceeded) || phase == string(corev1.PodFailed),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("webhook-ca-bundle"),
	// Runs after the internal TLS hook (Order 5) has put the current CA into values.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       webhookConfigurationSnapshot,
			ApiVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cert-manager-webhook"},
			},
			FilterFunc: applyWebhookConfigurationCAFilter,
		},
		{
			Name:       webhookDeploymentSnapshot,
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"webhook"},
			},
			NamespaceSelector: internal.NsSelector(),
			FilterFunc:        applyWebhookDeploymentFilter,
		},
		{
			Name:       webhookPodsSnapshot,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "webhook"},
			},
			NamespaceSelector: internal.NsSelector(),
			FilterFunc:        applyWebhookPodCAFilter,
		},
	},
}, buildWebhookCABundle)

func buildWebhookCABundle(_ context.Context, input *go_hook.HookInput) error {
	currentCA := input.Values.Get(webhookCAValuesPath).String()
	if currentCA == "" {
		// The TLS hook has not produced a CA yet; the template falls back to the CA itself.
		input.Values.Remove(caBundleValuesPath)
		input.Values.Set(caRotationActiveValuesPath, false)
		return nil
	}

	previous, err := trustedCAsFromSnapshot(input)
	if err != nil {
		// Without the trusted CAs the hook cannot keep the old one; the bundle degrades to the
		// current CA, which is the behaviour without this hook.
		input.Logger.Warn("cannot read CA bundle of the webhook configuration", "error", err)
	}

	bundle := []string{strings.TrimSpace(currentCA)}
	rotation := false

	if extra := previousCAs(previous, currentCA); len(extra) > 0 && !webhookRolledOut(input, currentCA) {
		bundle = append(bundle, extra...)
		rotation = true
	}

	input.Values.Set(caBundleValuesPath, strings.Join(bundle, "\n")+"\n")
	input.Values.Set(caRotationActiveValuesPath, rotation)
	return nil
}

func webhookRolledOut(input *go_hook.HookInput, currentCA string) bool {
	snaps := input.Snapshots.Get(webhookDeploymentSnapshot)
	if len(snaps) == 0 {
		// No Deployment means no pod serves an old certificate.
		return true
	}

	var state webhookDeploymentState
	if err := snaps[0].UnmarshalTo(&state); err != nil {
		input.Logger.Warn("cannot read the webhook Deployment state", "error", err)
		return false
	}

	checksum := caChecksum(currentCA)
	if !state.rolledOutWith(checksum) {
		return false
	}

	// A pod of an older template, terminating ones included, may still serve a certificate of the
	// previous CA. A pod that has finished serves nothing.
	for _, snap := range input.Snapshots.Get(webhookPodsSnapshot) {
		var pod webhookPodState
		if err := snap.UnmarshalTo(&pod); err != nil {
			input.Logger.Warn("cannot read the CA checksum of a webhook pod", "error", err)
			return false
		}
		if !pod.Finished && pod.CAChecksum != checksum {
			return false
		}
	}

	return true
}

func trustedCAsFromSnapshot(input *go_hook.HookInput) (string, error) {
	snaps := input.Snapshots.Get(webhookConfigurationSnapshot)
	if len(snaps) == 0 {
		return "", nil
	}

	var encoded string
	if err := snaps[0].UnmarshalTo(&encoded); err != nil {
		return "", err
	}

	return decodeBase64PEM(encoded)
}

// previousCAs returns the PEM certificates of trusted that differ from currentCA.
func previousCAs(trusted, currentCA string) []string {
	current := pemCertificates(currentCA)
	var out []string
	for _, cert := range pemCertificates(trusted) {
		known := false
		for _, c := range current {
			if bytes.Equal(c.Bytes, cert.Bytes) {
				known = true
				break
			}
		}
		if !known {
			out = append(out, strings.TrimSpace(string(pem.EncodeToMemory(cert))))
		}
	}
	return out
}

func pemCertificates(data string) []*pem.Block {
	var out []*pem.Block
	rest := []byte(data)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out
		}
		if block.Type == "CERTIFICATE" {
			out = append(out, block)
		}
	}
}

// caChecksum must match the checksum the webhook Deployment template renders into
// webhookCAChecksumAnnotation: sha256 of the CA as stored in values.
func caChecksum(ca string) string {
	sum := sha256.Sum256([]byte(ca))
	return hex.EncodeToString(sum[:])
}

func decodeBase64PEM(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode caBundle: %w", err)
	}
	return string(decoded), nil
}
