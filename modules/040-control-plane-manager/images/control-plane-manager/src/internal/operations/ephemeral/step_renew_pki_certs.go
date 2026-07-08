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
	"control-plane-manager/internal/certs"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var componentCertTree = map[controlplanev1alpha1.OperationComponent]map[pki.RootCertBaseName][]pki.LeafCertBaseName{
	controlplanev1alpha1.OperationComponentEtcd: {
		pki.EtcdCACertBaseName: {
			pki.EtcdServerCertBaseName,
			pki.EtcdPeerCertBaseName,
			pki.EtcdHealthcheckClientCertBaseName,
			pki.ApiserverEtcdClientCertBaseName,
		},
	},
	controlplanev1alpha1.OperationComponentKubeAPIServer: {
		pki.CACertBaseName: {
			pki.ApiserverCertBaseName,
			pki.ApiserverKubeletClientCertBaseName,
		},
		pki.EtcdCACertBaseName: {
			pki.ApiserverEtcdClientCertBaseName,
		},
		pki.FrontProxyCACertBaseName: {
			pki.FrontProxyClientCertBaseName,
		},
	},
}

type tenantPKIConfig struct {
	ClusterDomain       string
	ServiceSubnetCIDR   string
	APIServerCertSANs   []string
	EncryptionAlgorithm string
}

func (e *StepExecutor) renewPKICerts(ctx context.Context) operations.StepResult {
	const step = controlplanev1alpha1.StepRenewPKICerts

	certTree := componentCertTree[e.operation.Spec.Component]
	if len(certTree) == 0 {
		return operations.StepIsCompleted(step, "component has no renewable certificates")
	}

	cfg, err := e.loadTenantPKIConfig(ctx)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}

	advertiseAddress, err := e.apiserverClusterIP(ctx)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}

	secret, err := e.getPKISecret(ctx)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}
	pkiDir, err := os.MkdirTemp("", "vcp-renew-pki-*")
	if err != nil {
		return operations.StepHasFailed(step, fmt.Errorf("create temp pki dir: %w", err))
	}
	defer os.RemoveAll(pkiDir)

	if err := materializePKISecret(pkiDir, secret.Data); err != nil {
		return operations.StepHasFailed(step, err)
	}

	report, err := createComponentPKIBundle(e.tenantIdentity.Namespace, cfg, advertiseAddress, pkiDir, certTree)
	if err != nil {
		return operations.StepHasFailed(step, fmt.Errorf("renew certificates: %w", err))
	}

	base := secret.DeepCopy()
	renewed, err := applyRegeneratedLeafs(secret, pkiDir, report)
	if err != nil {
		return operations.StepHasFailed(step, err)
	}
	if renewed == 0 {
		return operations.StepIsCompleted(step, "all certificates valid, nothing to renew")
	}

	if err := e.client.Patch(ctx, secret, client.MergeFrom(base)); err != nil {
		return operations.StepHasFailed(step, fmt.Errorf("patch pki secret: %w", err))
	}

	return operations.StepIsCompleted(step, fmt.Sprintf("renewed %d certificate(s)", renewed))
}

func (e *StepExecutor) loadTenantPKIConfig(ctx context.Context) (tenantPKIConfig, error) {
	cfg := tenantPKIConfig{
		ClusterDomain:     e.clusterDomain,
		ServiceSubnetCIDR: e.serviceSubnetCIDR,
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{
		Namespace: e.tenantIdentity.Namespace,
		Name:      constants.VirtualRenderedConfigSecretName,
	}
	err := e.client.Get(ctx, key, secret)
	if apierrors.IsNotFound(err) {
		return cfg, nil
	}
	if err != nil {
		return tenantPKIConfig{}, fmt.Errorf("get tenant config secret %s: %w", key.Name, err)
	}

	if v, ok := secret.Data[constants.SecretKeyCertSANs]; ok && len(v) > 0 {
		cfg.APIServerCertSANs = strings.Split(string(v), ",")
	}
	if v, ok := secret.Data[constants.SecretKeyEncryptionAlgorithm]; ok && len(v) > 0 {
		cfg.EncryptionAlgorithm = string(v)
	}
	return cfg, nil
}

func createComponentPKIBundle(
	nodeName string,
	cfg tenantPKIConfig,
	advertiseAddress net.IP,
	pkiDir string,
	certTree map[pki.RootCertBaseName][]pki.LeafCertBaseName,
) (pki.PKIApplyReport, error) {
	if cfg.EncryptionAlgorithm != "" {
		return pki.CreatePKIBundle(
			nodeName, cfg.ClusterDomain, advertiseAddress, cfg.ServiceSubnetCIDR,
			pki.WithPKIDir(pkiDir),
			pki.WithCertTreeScheme(certTree),
			pki.WithAPIServerCertSANs(cfg.APIServerCertSANs),
			pki.WithEncryptionAlgorithmType(pkiconstants.EncryptionAlgorithmType(cfg.EncryptionAlgorithm)),
		)
	}
	return pki.CreatePKIBundle(
		nodeName, cfg.ClusterDomain, advertiseAddress, cfg.ServiceSubnetCIDR,
		pki.WithPKIDir(pkiDir),
		pki.WithCertTreeScheme(certTree),
		pki.WithAPIServerCertSANs(cfg.APIServerCertSANs),
	)
}

func (e *StepExecutor) apiserverClusterIP(ctx context.Context) (net.IP, error) {
	svc := &corev1.Service{}
	key := client.ObjectKey{Namespace: e.tenantIdentity.Namespace, Name: "kube-apiserver"}
	if err := e.client.Get(ctx, key, svc); err != nil {
		return nil, fmt.Errorf("get apiserver service: %w", err)
	}
	ip := net.ParseIP(svc.Spec.ClusterIP)
	if ip == nil {
		return nil, fmt.Errorf("apiserver service has invalid ClusterIP %q", svc.Spec.ClusterIP)
	}
	return ip, nil
}

func (e *StepExecutor) getPKISecret(ctx context.Context) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := client.ObjectKey{
		Namespace: e.tenantIdentity.Namespace,
		Name:      constants.VirtualPKISecretName,
	}
	if err := e.client.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("get pki secret %s: %w", key.Name, err)
	}
	return secret, nil
}

func materializePKISecret(pkiDir string, data map[string][]byte) error {
	for flatKey, relPath := range certs.VirtualCertsFileLayout() {
		content, ok := data[flatKey]
		if !ok {
			continue
		}
		dst := filepath.Join(pkiDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return fmt.Errorf("create dir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(dst, content, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

func applyRegeneratedLeafs(secret *corev1.Secret, pkiDir string, report pki.PKIApplyReport) (int, error) {
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	count := 0
	for _, entry := range report.Entries {
		if entry.Kind != pki.PKIEntryKindLeafCert {
			continue
		}
		if entry.Action != pki.PKIActionWrittenCreated && entry.Action != pki.PKIActionWrittenRegenerated {
			continue
		}

		secretBase := pki.FlatBaseName(entry.Name)
		for _, ext := range []string{".crt", ".key"} {
			content, err := os.ReadFile(filepath.Join(pkiDir, entry.Name+ext))
			if err != nil {
				return 0, fmt.Errorf("read renewed %s%s: %w", entry.Name, ext, err)
			}
			secret.Data[secretBase+ext] = content
		}
		count++
	}
	return count, nil
}
