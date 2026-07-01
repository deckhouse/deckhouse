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

package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var componentKubeconfigFiles = map[controlplanev1alpha1.OperationComponent][]kubeconfig.File{
	controlplanev1alpha1.OperationComponentKubeControllerManager: {kubeconfig.ControllerManager},
	controlplanev1alpha1.OperationComponentKubeScheduler:         {kubeconfig.Scheduler},
}

func (e *StepExecutor) certObserve(ctx context.Context) operations.StepResult {
	const step = controlplanev1alpha1.StepCertObserve

	certExpiry := map[string]metav1.Time{}

	if err := e.observeLeafCertExpirations(ctx, certExpiry); err != nil {
		return operations.StepHasFailed(step, err)
	}
	if err := e.observeKubeconfigExpirations(ctx, certExpiry); err != nil {
		return operations.StepHasFailed(step, err)
	}

	if len(certExpiry) == 0 {
		return operations.StepIsCompleted(step, "no certificates to observe")
	}

	state := &controlplanev1alpha1.ObservedComponentState{
		CertificatesExpirationTime: certExpiry,
	}

	return operations.StepIsCompleted(step,
		fmt.Sprintf("observed %d certificate(s)", len(certExpiry)), func(operation *controlplanev1alpha1.ControlPlaneOperation) {
			operation.Status.ObservedState = state
		})
}

func (e *StepExecutor) observeLeafCertExpirations(ctx context.Context, out map[string]metav1.Time) error {
	leafs := componentLeafCerts(e.operation.Spec.Component)
	if len(leafs) == 0 {
		return nil
	}

	secret, err := e.getPKISecret(ctx)
	if err != nil {
		return err
	}

	for _, leaf := range leafs {
		key := strings.ReplaceAll(string(leaf), "/", "-") + ".crt"
		pemBytes, ok := secret.Data[key]
		if !ok {
			return fmt.Errorf("pki secret missing %q", key)
		}
		notAfter, err := certNotAfter(pemBytes)
		if err != nil {
			return fmt.Errorf("parse certificate %s: %w", key, err)
		}
		out[key] = metav1.NewTime(notAfter)
	}
	return nil
}

func (e *StepExecutor) observeKubeconfigExpirations(ctx context.Context, out map[string]metav1.Time) error {
	files := componentKubeconfigFiles[e.operation.Spec.Component]
	if len(files) == 0 {
		return nil
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{
		Namespace: e.tenantIdentity.Namespace,
		Name:      e.tenantIdentity.Namespace + "-kubeconfig",
	}
	if err := e.client.Get(ctx, key, secret); err != nil {
		return fmt.Errorf("get kubeconfig secret %s: %w", key.Name, err)
	}

	for _, file := range files {
		raw, ok := secret.Data[string(file)]
		if !ok {
			return fmt.Errorf("kubeconfig secret missing %q", file)
		}
		notAfter, err := kubeconfigClientCertNotAfter(raw)
		if err != nil {
			return fmt.Errorf("parse kubeconfig %s: %w", file, err)
		}
		out[string(file)] = metav1.NewTime(notAfter)
	}
	return nil
}

func componentLeafCerts(component controlplanev1alpha1.OperationComponent) []pki.LeafCertName {
	tree := componentCertTree[component]
	if len(tree) == 0 {
		return nil
	}
	var leafs []pki.LeafCertName
	for _, names := range tree {
		leafs = append(leafs, names...)
	}
	return leafs
}

func kubeconfigClientCertNotAfter(raw []byte) (time.Time, error) {
	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("load kubeconfig: %w", err)
	}

	kubeContext, ok := cfg.Contexts[cfg.CurrentContext]
	if !ok {
		return time.Time{}, fmt.Errorf("current context %q not found", cfg.CurrentContext)
	}
	authInfo, ok := cfg.AuthInfos[kubeContext.AuthInfo]
	if !ok {
		return time.Time{}, fmt.Errorf("auth info %q not found", kubeContext.AuthInfo)
	}
	if len(authInfo.ClientCertificateData) == 0 {
		return time.Time{}, fmt.Errorf("kubeconfig has no embedded client-certificate-data")
	}

	return certNotAfter(authInfo.ClientCertificateData)
}

func certNotAfter(pemBytes []byte) (time.Time, error) {
	certs, err := certutil.ParseCertsPEM(pemBytes)
	if err != nil {
		return time.Time{}, err
	}
	if len(certs) == 0 {
		return time.Time{}, fmt.Errorf("no certificate found in PEM data")
	}
	return certs[0].NotAfter, nil
}
