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

package kubeconfig

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

var logger = log.Default().Named("kubeconfig")

type fileSpec struct {
	ClusterName             string
	APIServer               string
	ClientName              string
	ClientCertOrganizations []string
	ClientCertNotAfter      time.Time
	CACert                  *x509.Certificate
	CAKey                   crypto.Signer
	EncryptionAlgorithm     constants.EncryptionAlgorithmType
}

func CreateKubeconfigFiles(files []File, options ...option) error {
	logger.Info("creating kubeconfig files for control-plane")

	opt, err := prepareOptions(options...)
	if err != nil {
		return fmt.Errorf("failed to prepare Options: %w", err)
	}

	for _, file := range files {
		if err := createKubeConfigFile(file, opt); err != nil {
			return fmt.Errorf("failed to create kubeconfig file %q: %w", file, err)
		}
	}

	return nil
}

func createKubeConfigFile(file File, opt *options) error {
	fileSpec, err := getFileSpec(file, opt)
	if err != nil {
		return fmt.Errorf("failed to get spec for %q: %w", file, err)
	}

	config, err := buildConfig(fileSpec)
	if err != nil {
		return fmt.Errorf("failed to build kube config for %q: %w", file, err)
	}

	kubeConfigFilePath := filepath.Join(opt.OutDir, string(file))

	if err := writeKubeConfigFileIfNeeded(kubeConfigFilePath, config); err != nil {
		return err
	}

	return nil
}

func getFileSpec(kind File, opt *options) (*fileSpec, error) {
	switch kind {
	case Admin:
		return &fileSpec{
			ClusterName:             opt.ClusterName,
			APIServer:               opt.ControlPlaneEndpoint,
			ClientName:              "kubernetes-admin",
			ClientCertOrganizations: []string{"kubeadm:cluster-admins"},
			ClientCertNotAfter:      opt.CertProvider.NotAfter(),
			CACert:                  opt.CertProvider.CACert(),
			CAKey:                   opt.CertProvider.CAKey(),
			EncryptionAlgorithm:     opt.EncryptionAlgorithm,
		}, nil
	case SuperAdmin:
		return &fileSpec{
			ClusterName:             opt.ClusterName,
			APIServer:               opt.ControlPlaneEndpoint,
			ClientName:              "kubernetes-super-admin",
			ClientCertOrganizations: []string{"system:masters"},
			ClientCertNotAfter:      opt.CertProvider.NotAfter(),
			CACert:                  opt.CertProvider.CACert(),
			CAKey:                   opt.CertProvider.CAKey(),
			EncryptionAlgorithm:     opt.EncryptionAlgorithm,
		}, nil
	case ControllerManager:
		return &fileSpec{
			ClusterName:         opt.ClusterName,
			APIServer:           opt.LocalAPIEndpoint,
			ClientName:          "system:kube-controller-manager",
			ClientCertNotAfter:  opt.CertProvider.NotAfter(),
			CACert:              opt.CertProvider.CACert(),
			CAKey:               opt.CertProvider.CAKey(),
			EncryptionAlgorithm: opt.EncryptionAlgorithm,
		}, nil
	case Scheduler:
		return &fileSpec{
			ClusterName:         opt.ClusterName,
			APIServer:           opt.LocalAPIEndpoint,
			ClientName:          "system:kube-scheduler",
			ClientCertNotAfter:  opt.CertProvider.NotAfter(),
			CACert:              opt.CertProvider.CACert(),
			CAKey:               opt.CertProvider.CAKey(),
			EncryptionAlgorithm: opt.EncryptionAlgorithm,
		}, nil
	case Kubelet:
		return &fileSpec{
			ClusterName:             fmt.Sprintf("%s%s", "system:node:", opt.NodeName),
			APIServer:               opt.ControlPlaneEndpoint,
			ClientName:              "kubernetes-super-admin",
			ClientCertOrganizations: []string{"system:masters"},
			ClientCertNotAfter:      opt.CertProvider.NotAfter(),
			CACert:                  opt.CertProvider.CACert(),
			CAKey:                   opt.CertProvider.CAKey(),
			EncryptionAlgorithm:     opt.EncryptionAlgorithm,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported kind %s", kind)
	}
}

func buildConfig(spec *fileSpec) (*clientcmdapi.Config, error) {
	clientCertConfig := newClientCertConfig(spec)

	clientCert, clientKey, err := pkiutil.NewCertAndKey(spec.CACert, spec.CAKey, clientCertConfig)
	if err != nil {
		return nil, fmt.Errorf("failure while creating %q client certificate: %w", spec.ClientName, err)
	}
	encodedClientKey, err := keyutil.MarshalPrivateKeyToPEM(clientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key to PEM: %w", err)
	}

	contextName := fmt.Sprintf("%s@%s", spec.ClientName, spec.ClusterName)
	config := &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			spec.ClusterName: {
				Server:                   spec.APIServer,
				CertificateAuthorityData: pkiutil.EncodeCertificate(spec.CACert),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  spec.ClusterName,
				AuthInfo: spec.ClientName,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			spec.ClientName: {
				ClientCertificateData: pkiutil.EncodeCertificate(clientCert),
				ClientKeyData:         encodedClientKey,
			},
		},
		CurrentContext: contextName,
	}

	return config, nil
}

func newClientCertConfig(spec *fileSpec) pkiutil.CertConfig {
	return pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   spec.ClientName,
			Organization: spec.ClientCertOrganizations,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		NotAfter:            spec.ClientCertNotAfter,
		EncryptionAlgorithm: spec.EncryptionAlgorithm,
	}
}

func writeKubeConfigFileIfNeeded(kubeConfigFilePath string, config *clientcmdapi.Config) error {
	err := validateCurrentKubeConfig(kubeConfigFilePath, config)
	if err == nil {
		logger.Info("Using existing kubeconfig file", slog.String("path", kubeConfigFilePath))
		return nil
	}

	logger.Info("Writing new kubeconfig file", slog.String("path", kubeConfigFilePath), slog.String("reason", err.Error()))

	if err := clientcmd.WriteToFile(*config, kubeConfigFilePath); err != nil {
		return fmt.Errorf("failed to write kubeconfig %q: %w", kubeConfigFilePath, err)
	}

	return nil
}

func validateCurrentKubeConfig(kubeConfigFilePath string, desiredConfig *clientcmdapi.Config) error {
	if _, err := os.Stat(kubeConfigFilePath); err != nil {
		return fmt.Errorf("kubeconfig file not found: %w", err)
	}

	currentConfig, err := clientcmd.LoadFromFile(kubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	currentCtx, ok := currentConfig.Contexts[currentConfig.CurrentContext]
	if !ok {
		return fmt.Errorf("current context %q not found", currentConfig.CurrentContext)
	}

	currentCluster, ok := currentConfig.Clusters[currentCtx.Cluster]
	if !ok {
		return fmt.Errorf("cluster %q not found", currentCtx.Cluster)
	}

	desiredCtx := desiredConfig.Contexts[desiredConfig.CurrentContext]
	desiredCluster := desiredConfig.Clusters[desiredCtx.Cluster]

	if currentCluster.Server != desiredCluster.Server {
		return fmt.Errorf("kubeconfig address field changed: expected %q, got %q", desiredCluster.Server, currentCluster.Server)
	}

	if !bytes.Equal(bytes.TrimSpace(currentCluster.CertificateAuthorityData), bytes.TrimSpace(desiredCluster.CertificateAuthorityData)) {
		return fmt.Errorf("CA certificate changed")
	}

	currentAuth, ok := currentConfig.AuthInfos[currentCtx.AuthInfo]
	if !ok {
		return fmt.Errorf("auth info %q not found", currentCtx.AuthInfo)
	}

	certData := currentAuth.ClientCertificateData
	if len(certData) == 0 {
		return fmt.Errorf("client-certificate-data field is empty")
	}

	block, _ := pem.Decode(certData)
	if block == nil || len(block.Bytes) == 0 {
		return fmt.Errorf("cannot PEM decode client-certificate-data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("cannot parse certificate from client-certificate-data: %w", err)
	}

	if pkiutil.CertificateExpiresSoon(cert, 30*24*time.Hour) {
		return fmt.Errorf("client certificate is expiring in less than 30 days")
	}

	return nil
}
