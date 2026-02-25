package kubeconfig

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/pki"
	"github.com/deckhouse/deckhouse/pkg/log"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

type FileSpec struct {
	ClusterName             string
	APIServer               string
	ClientName              string
	ClientCertOrganizations []string
	ClientCertNotAfter      time.Time
	CACert                  *x509.Certificate
	CAKey                   crypto.Signer
	EncryptionAlgorithm     constants.EncryptionAlgorithmType
}

func CreateControlPlaneKubeConfigFiles(options ...Option) error {
	log.Info("creating kubeconfig files for control-plane")

	opt, err := buildOptions(options...)
	if err != nil {
		return fmt.Errorf("failed to build Options: %w", err)
	}

	files := []File{
		SuperAdmin,
		Admin,
		Scheduler,
		ControllerManager,
	}

	return createKubeConfigFiles(opt, files...)
}

func createKubeConfigFiles(opt *Options, files ...File) error {
	for _, file := range files {
		fileSpec, err := getFileSpec(file, opt)
		if err != nil {
			return fmt.Errorf("failed to get spec for file %s: %w", file, err)
		}

		config, err := buildConfig(fileSpec)
		if err != nil {
			return fmt.Errorf("failed to build kube config for %s: %w", file, err)
		}

		if err = clientcmd.WriteToFile(*config, "filename"); err != nil {
			
		}
	}

	return nil
}

func getFileSpec(kind File, opt *Options) (*FileSpec, error) {
	switch kind {
	case Admin:
		return &FileSpec{
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
		return &FileSpec{
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
		return &FileSpec{
			ClusterName:         opt.ClusterName,
			APIServer:           opt.LocalAPIEndpoint,
			ClientName:          "system:kube-controller-manager",
			ClientCertNotAfter:  opt.CertProvider.NotAfter(),
			CACert:              opt.CertProvider.CACert(),
			CAKey:               opt.CertProvider.CAKey(),
			EncryptionAlgorithm: opt.EncryptionAlgorithm,
		}, nil
	case Scheduler:
		return &FileSpec{
			ClusterName:         opt.ClusterName,
			APIServer:           opt.LocalAPIEndpoint,
			ClientName:          "system:kube-scheduler",
			ClientCertNotAfter:  opt.CertProvider.NotAfter(),
			CACert:              opt.CertProvider.CACert(),
			CAKey:               opt.CertProvider.CAKey(),
			EncryptionAlgorithm: opt.EncryptionAlgorithm,
		}, nil
	case Kubelet:
		nodeName := ""
		return &FileSpec{
			ClusterName:             fmt.Sprintf("%s%s", "system:node:", nodeName),
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

func buildConfig(spec *FileSpec) (*clientcmdapi.Config, error) {
	clientCertConfig := newClientCertConfig(spec)

	clientCert, clientKey, err := pki.NewCertAndKey(spec.CACert, spec.CAKey, &clientCertConfig)
	if err != nil {
		return nil, fmt.Errorf("failure while creating %s client certificate: %w", spec.ClientName, err)
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
				CertificateAuthorityData: pki.EncodeCertificate(spec.CACert),
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
				ClientCertificateData: pki.EncodeCertificate(clientCert),
				ClientKeyData:         encodedClientKey,
			},
		},
		CurrentContext: contextName,
	}

	return config, nil
}

func newClientCertConfig(spec *FileSpec) pki.CertConfig {
	return pki.CertConfig{
		Config: certutil.Config{
			CommonName:   spec.ClientName,
			Organization: spec.ClientCertOrganizations,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		NotAfter:            spec.ClientCertNotAfter,
		EncryptionAlgorithm: spec.EncryptionAlgorithm,
	}
}
