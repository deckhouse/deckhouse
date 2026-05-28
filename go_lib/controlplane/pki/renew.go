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
	"time"

	certutil "k8s.io/client-go/util/cert"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

type LeafCertificateInfo struct {
	Name        LeafCertName
	Description string
}

type RenewOption func(*renewOptions)

type renewOptions struct {
	certificatesDir  string
	leafCertificates []LeafCertName
}

// WithRenewDir overrides the PKI directory used by Renew*.
// Defaults to constants.DefaultCertificatesDir (e.g. /etc/kubernetes/pki).
func WithRenewDir(dir string) RenewOption {
	return func(o *renewOptions) {
		o.certificatesDir = dir
	}
}

// WithRenewLeafs restricts RenewCertificates to the provided leaf names.
// When empty, the full DefaultLeafCertificates() inventory is used.
func WithRenewLeafs(names ...LeafCertName) RenewOption {
	return func(o *renewOptions) {
		o.leafCertificates = append(o.leafCertificates, names...)
	}
}

func newRenewOptions(opts ...RenewOption) *renewOptions {
	o := &renewOptions{certificatesDir: constants.DefaultCertificatesDir}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// PKIRenewReport describes per-certificate outcomes of a RenewCertificates call.
type PKIRenewReport struct {
	Entries []PKIRenewEntry
}

// PKIRenewEntry is one row of PKIRenewReport.
// Err == nil means the cert was renewed successfully; otherwise Err is a sentinel describing what happened:
//   - *MissingError   when the leaf cert file is absent (skippable)
//   - *CAExternalError when the CA private key is absent (skippable)
//   - *CAExpiredError when the signing CA has expired (renewal is pointless and the cert is left untouched)
//   - any other error (wrapped) for IO/permissions/signing failures
type PKIRenewEntry struct {
	Name      LeafCertName
	Path      string
	Authority RootCertName
	Err       error
}

func (r *PKIRenewReport) add(name LeafCertName, path string, authority RootCertName, err error) {
	r.Entries = append(r.Entries, PKIRenewEntry{
		Name:      name,
		Path:      path,
		Authority: authority,
		Err:       err,
	})
}

func certConfigFromX509(cert *x509.Certificate) certConfig {
	return certConfig{
		Config: certutil.Config{
			CommonName:   cert.Subject.CommonName,
			Organization: cert.Subject.Organization,
			AltNames: certutil.AltNames{
				DNSNames: cert.DNSNames,
				IPs:      cert.IPAddresses,
			},
			Usages: cert.ExtKeyUsage,
		},
		EncryptionAlgorithm: pkiutil.DetectEncryptionAlgorithm(cert),
	}
}

func caForLeaf(name LeafCertName) (RootCertName, bool) {
	for caName, leafNames := range defaultCertTreeScheme {
		for _, leafName := range leafNames {
			if leafName == name {
				return caName, true
			}
		}
	}
	return "", false
}

func renewLeafCert(pkiDir string, name LeafCertName) error {
	caName, ok := caForLeaf(name)
	if !ok {
		return fmt.Errorf("unknown leaf certificate %q", name)
	}

	certFile := certPath(pkiDir, string(name))
	oldCert, err := pkiutil.LoadCert(certFile)
	if err != nil {
		if isNotExistError(err) {
			return &MissingError{BaseName: string(name)}
		}
		return fmt.Errorf("load cert %q: %w", name, err)
	}

	caCertFile := certPath(pkiDir, string(caName))
	caCert, err := pkiutil.LoadCert(caCertFile)
	if err != nil {
		if isNotExistError(err) {
			return fmt.Errorf("CA cert %q not found", caName)
		}
		return fmt.Errorf("load CA cert %q: %w", caName, err)
	}

	if time.Now().After(caCert.NotAfter) {
		return &CAExpiredError{CAName: string(caName), ExpiredAt: caCert.NotAfter}
	}

	caKey, err := pkiutil.LoadKey(keyPath(pkiDir, string(caName)))
	if err != nil {
		if isNotExistError(err) {
			return &CAExternalError{CAName: string(caName)}
		}
		return fmt.Errorf("load CA key %q: %w", caName, err)
	}

	cfg := certConfigFromX509(oldCert)
	cfg.CertificateValidityPeriod = constants.CertificateValidityPeriod

	newKey, err := pkiutil.NewPrivateKey(cfg.EncryptionAlgorithm)
	if err != nil {
		return fmt.Errorf("generate new key for cert %q: %w", name, err)
	}

	newCert, err := pkiutil.NewSignedCert(cfg, newKey, caCert, caKey)
	if err != nil {
		return fmt.Errorf("sign cert %q: %w", name, err)
	}
	if err := writeCertAndKey(pkiDir, string(name), newCert, newKey); err != nil {
		return fmt.Errorf("write cert %q: %w", name, err)
	}
	return nil
}

// RenewLeafCert renews a leaf certificate by re-signing it with the same private key.
// All Subject/SAN/Usage/Algorithm fields are preserved from the current certificate file.
// The new certificate is issued with constants.CertificateValidityPeriod (1 year).
// Sentinel errors:
//   - *MissingError  — leaf cert file absent (skippable)
//   - *CAExternalError   — CA key absent (skippable)
//   - *CAExpiredError    — CA cert expired (hard stop; renewal is pointless)
func RenewCertificate(name LeafCertName, opts ...RenewOption) error {
	o := newRenewOptions(opts...)
	return renewLeafCert(o.certificatesDir, name)
}

// RenewCertificates iterates the inventory (or the subset chosen via WithRenewLeafs) and renews each leaf certificate in turn.
// Iteration never aborts — the caller iterates report.Entries and decides per-entry what to do.
func RenewCertificates(opts ...RenewOption) PKIRenewReport {
	o := newRenewOptions(opts...)

	inventory := selectLeafs(o.leafCertificates)

	var report PKIRenewReport
	for _, info := range inventory {
		authority, _ := caForLeaf(info.Name)
		path := certPath(o.certificatesDir, string(info.Name))

		report.add(info.Name, path, authority, renewLeafCert(o.certificatesDir, info.Name))
	}

	return report
}

// defaultLeafCertificates returns the canonical list of renewable control-plane leaf certificates.
func DefaultLeafCertificates() []LeafCertificateInfo {
	return []LeafCertificateInfo{
		{ApiserverCertName, "certificate for serving the Kubernetes API"},
		{ApiserverKubeletClientCertName, "certificate for the API server to connect to kubelet"},
		{ApiserverEtcdClientCertName, "certificate the apiserver uses to access etcd"},
		{FrontProxyClientCertName, "certificate for the front proxy client"},
		{EtcdServerCertName, "certificate for serving etcd"},
		{EtcdPeerCertName, "certificate for etcd nodes to communicate with each other"},
		{EtcdHealthcheckClientCertName, "certificate for liveness probes to healthcheck etcd"},
	}
}

// selectLeafs returns the inventory with only the given names, preserving the canonical DefaultLeafCertificates() order.
// When names is empty, returned default inventory.
func selectLeafs(names []LeafCertName) []LeafCertificateInfo {
	full := DefaultLeafCertificates()
	if len(names) == 0 {
		return full
	}

	wanted := make(map[LeafCertName]struct{}, len(names))
	for _, n := range names {
		wanted[n] = struct{}{}
	}

	result := make([]LeafCertificateInfo, 0, len(names))
	for _, info := range full {
		if _, ok := wanted[info.Name]; ok {
			result = append(result, info)
		}
	}
	return result
}
