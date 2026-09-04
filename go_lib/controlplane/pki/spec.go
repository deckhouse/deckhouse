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

package pki

import (
	"crypto/x509"
	"fmt"
	"net"

	certutil "k8s.io/client-go/util/cert"
)

// rootCertSpec combines a CA's own spec with the specs of all leaf certificates it signs.
type rootCertSpec struct {
	certSpec[RootCertBaseName]
	leafCerts []certSpec[LeafCertBaseName]
}

// certSpec holds the information needed to locate and generate one certificate on disk.
// BaseName is the file path relative to PKIDir, without extension (e.g. "etcd/server").
// BuildConfig constructs the full certificate configuration from the runtime config.
type certSpec[T LeafCertBaseName | RootCertBaseName] struct {
	BaseName    string
	BuildConfig func(cfg config) certConfig
}

// renderCertSpecTree converts a certTreeScheme (names only) into a certSpecTree (full specs).
// It resolves each CA name and leaf name to their respective certSpec definitions.
func renderCertSpecTree(treeScheme certTreeScheme) certSpecTree {
	tree := make(map[RootCertBaseName]rootCertSpec, len(treeScheme))

	for rootCertName, leafCertNames := range treeScheme {
		rootCertSpec := getRootCertSpec(rootCertName)

		for _, leafCertName := range leafCertNames {
			rootCertSpec.leafCerts = append(rootCertSpec.leafCerts, getLeafCertSpec(leafCertName))
		}

		tree[rootCertName] = rootCertSpec
	}

	return tree
}

func getRootCertSpec(name RootCertBaseName) rootCertSpec {
	switch name {
	case CACertBaseName:
		return rootCertSpec{
			certSpec: certSpec[RootCertBaseName]{
				BaseName: string(CACertBaseName),
				BuildConfig: func(cfg config) certConfig {
					return certConfig{
						Config: certutil.Config{
							CommonName: "kubernetes",
						},
						EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
						CertificateValidityPeriod: cfg.CACertValidityPeriod,
					}
				},
			},
		}
	case FrontProxyCACertBaseName:
		return rootCertSpec{
			certSpec: certSpec[RootCertBaseName]{
				BaseName: string(FrontProxyCACertBaseName),
				BuildConfig: func(cfg config) certConfig {
					return certConfig{
						Config: certutil.Config{
							CommonName: "front-proxy-ca",
						},
						EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
						CertificateValidityPeriod: cfg.CACertValidityPeriod,
					}
				},
			},
		}
	case EtcdCACertBaseName:
		return rootCertSpec{
			certSpec: certSpec[RootCertBaseName]{
				BaseName: string(EtcdCACertBaseName),
				BuildConfig: func(cfg config) certConfig {
					return certConfig{
						Config: certutil.Config{
							CommonName: "etcd-ca",
						},
						EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
						CertificateValidityPeriod: cfg.CACertValidityPeriod,
					}
				},
			},
		}
	default:
		panic(fmt.Sprintf("unknown certName %s", name))
	}
}

func getLeafCertSpec(name LeafCertBaseName) certSpec[LeafCertBaseName] {
	switch name {
	case ApiserverCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(ApiserverCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				domain := cfg.DNSDomain

				altNames := certutil.AltNames{
					DNSNames: []string{
						"kubernetes",
						"kubernetes.default",
						"kubernetes.default.svc",
						fmt.Sprintf("kubernetes.default.svc.%s", domain),
					},
					IPs: []net.IP{
						net.IPv4(127, 0, 0, 1),
					},
				}

				if cfg.NodeName != "" {
					if net.ParseIP(cfg.NodeName) == nil {
						altNames.DNSNames = append(altNames.DNSNames, cfg.NodeName)
					}
				}

				if cfg.AdvertiseAddress != nil {
					altNames.IPs = append(altNames.IPs, cfg.AdvertiseAddress)
				}

				if cfg.ServiceCIDR != "" {
					if ip, err := firstIPInCIDR(cfg.ServiceCIDR); err == nil {
						altNames.IPs = append(altNames.IPs, ip)
					}
				}

				if cfg.ControlPlaneEndpoint != "" {
					host := stripPort(cfg.ControlPlaneEndpoint)
					if ip := net.ParseIP(host); ip != nil {
						altNames.IPs = append(altNames.IPs, ip)
					} else {
						altNames.DNSNames = append(altNames.DNSNames, host)
					}
				}

				for _, san := range cfg.APIServerCertSANs {
					if ip := net.ParseIP(san); ip != nil {
						altNames.IPs = append(altNames.IPs, ip)
					} else {
						altNames.DNSNames = append(altNames.DNSNames, san)
					}
				}

				return certConfig{
					Config: certutil.Config{
						CommonName: "kube-apiserver",
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
						AltNames:   altNames,
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case ApiserverKubeletClientCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(ApiserverKubeletClientCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				return certConfig{
					Config: certutil.Config{
						CommonName: "kube-apiserver-kubelet-client",
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case FrontProxyClientCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(FrontProxyClientCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				return certConfig{
					Config: certutil.Config{
						CommonName: "front-proxy-client",
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case EtcdServerCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(EtcdServerCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				altNames := certutil.AltNames{
					DNSNames: []string{cfg.NodeName, "localhost"},
					IPs: []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("::1"),
					},
				}
				if cfg.AdvertiseAddress != nil {
					altNames.IPs = append(altNames.IPs, cfg.AdvertiseAddress)
				}
				for _, san := range cfg.EtcdServerCertSANs {
					if ip := net.ParseIP(san); ip != nil {
						altNames.IPs = append(altNames.IPs, ip)
					} else {
						altNames.DNSNames = append(altNames.DNSNames, san)
					}
				}
				return certConfig{
					Config: certutil.Config{
						CommonName: cfg.NodeName,
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
						AltNames:   altNames,
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case EtcdPeerCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(EtcdPeerCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				altNames := certutil.AltNames{
					DNSNames: []string{cfg.NodeName, "localhost"},
					IPs: []net.IP{
						net.ParseIP("127.0.0.1"),
						net.ParseIP("::1"),
					},
				}
				if cfg.AdvertiseAddress != nil {
					altNames.IPs = append(altNames.IPs, cfg.AdvertiseAddress)
				}
				for _, san := range cfg.EtcdPeerCertSANs {
					if ip := net.ParseIP(san); ip != nil {
						altNames.IPs = append(altNames.IPs, ip)
					} else {
						altNames.DNSNames = append(altNames.DNSNames, san)
					}
				}
				return certConfig{
					Config: certutil.Config{
						CommonName: cfg.NodeName,
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
						AltNames:   altNames,
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case EtcdHealthcheckClientCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(EtcdHealthcheckClientCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				return certConfig{
					Config: certutil.Config{
						CommonName: "kube-etcd-healthcheck-client",
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	case ApiserverEtcdClientCertBaseName:
		return certSpec[LeafCertBaseName]{
			BaseName: string(ApiserverEtcdClientCertBaseName),
			BuildConfig: func(cfg config) certConfig {
				return certConfig{
					Config: certutil.Config{
						CommonName: "kube-apiserver-etcd-client",
						Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
					},
					EncryptionAlgorithm:       cfg.EncryptionAlgorithmType,
					CertificateValidityPeriod: cfg.CertValidityPeriod,
				}
			},
		}
	default:
		panic(fmt.Sprintf("unknown certName %s", name))
	}
}
