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
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

type fileInfo struct {
	File        File
	Description string
}

// FileDescription returns the human-readable description for the given kubeconfig file.
// Falls back to the string representation of file when no description is found.
func FileDescription(file File) string {
	for _, info := range defaultRenewableFiles() {
		if info.File == file {
			return info.Description
		}
	}
	return string(file)
}

// defaultRenewableFiles returns the canonical list of renewable kubeconfig files.
// The kubelet.conf entry is intentionally omitted — kubelet manages its own client certificate rotation.
func defaultRenewableFiles() []fileInfo {
	return []fileInfo{
		{Admin, "certificate embedded in the kubeconfig file for the admin to use"},
		{SuperAdmin, "certificate embedded in the kubeconfig file for the super-admin"},
		{ControllerManager, "certificate embedded in the kubeconfig file for the controller manager to use"},
		{Scheduler, "certificate embedded in the kubeconfig file for the scheduler to use"},
	}
}

// RenewOption configures RenewClientCert and RenewClientCerts.
type RenewOption func(*renewOptions)

type renewOptions struct {
	kubeconfigDir string
	pkiDir        string
	files         []File
	dryRun        bool
}

// WithRenewKubeconfigDir overrides the directory containing kubeconfig files.
// Defaults to DefaultOutDir (/etc/kubernetes).
func WithRenewKubeconfigDir(dir string) RenewOption {
	return func(o *renewOptions) {
		o.kubeconfigDir = dir
	}
}

// WithRenewPKIDir overrides the directory containing the CA cert/key used to sign the new client certificates.
// Defaults to constants.DefaultCertificatesDir (/etc/kubernetes/pki).
func WithRenewPKIDir(dir string) RenewOption {
	return func(o *renewOptions) {
		o.pkiDir = dir
	}
}

// WithRenewFiles restricts RenewClientCerts to the provided kubeconfig files.
// When empty, the full default renewable files inventory is used.
func WithRenewFiles(files ...File) RenewOption {
	return func(o *renewOptions) {
		o.files = append(o.files, files...)
	}
}

// WithDryRun runs all renewal checks and signing in memory but skips writing the new kubeconfig to disk.
// The returned error contract is unchanged.
func WithDryRun() RenewOption {
	return func(o *renewOptions) {
		o.dryRun = true
	}
}

func newRenewOptions(opts ...RenewOption) *renewOptions {
	o := &renewOptions{
		kubeconfigDir: DefaultOutDir,
		pkiDir:        constants.DefaultCertificatesDir,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// KubeconfigRenewReport describes per-file outcomes of a RenewClientCerts call.
type KubeconfigRenewReport struct {
	Entries []KubeconfigRenewEntry
}

// KubeconfigRenewEntry is one row of KubeconfigRenewReport.
// Err == nil means the client cert was re-signed; otherwise Err is a sentinel describing what happened:
//   - *MissingError    when the kubeconfig file is absent (skippable)
//   - *CAExternalError when the CA private key is absent (skippable)
//   - *CAExpiredError  when the signing CA has expired (renewal is pointless and the kubeconfig is left untouched)
//   - any other error (wrapped) for IO/permissions/signing failures
type KubeconfigRenewEntry struct {
	File File
	Path string
	Err  error
}

func (r *KubeconfigRenewReport) add(file File, path string, err error) {
	r.Entries = append(r.Entries, KubeconfigRenewEntry{
		File: file,
		Path: path,
		Err:  err,
	})
}

// clientCertConfigFromX509 extracts CN/Organization/Usages from the embedded client certificate.
func clientCertConfigFromX509(cert *x509.Certificate) pkiutil.CertConfig {
	return pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   cert.Subject.CommonName,
			Organization: cert.Subject.Organization,
			Usages:       cert.ExtKeyUsage,
		},
		EncryptionAlgorithm:       pkiutil.DetectEncryptionAlgorithm(cert),
		CertificateValidityPeriod: constants.CertificateValidityPeriod,
	}
}

// renewClientCert unconditionally re-signs the client certificate embedded in the given kubeconfig file.
// All other kubeconfig fields are preserved. CA cert and key are loaded from pkiDir.
func renewClientCert(kubeconfigDir, pkiDir string, file File, dryRun bool) error {
	path := filepath.Join(kubeconfigDir, string(file))

	oldCert, err := loadClientCertificate(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &MissingError{File: file}
		}
		return err
	}

	caCert, err := pkiutil.LoadCert(filepath.Join(pkiDir, "ca.crt"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &CAMissingError{CAName: "ca"}
		}
		return fmt.Errorf("load CA cert: %w", err)
	}

	if time.Now().After(caCert.NotAfter) {
		return &CAExpiredError{CAName: "ca", ExpiredAt: caCert.NotAfter}
	}

	caKey, err := pkiutil.LoadKey(filepath.Join(pkiDir, "ca.key"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &CAExternalError{CAName: "ca"}
		}
		return fmt.Errorf("load CA key: %w", err)
	}

	cfg := clientCertConfigFromX509(oldCert)

	newCert, newKey, err := pkiutil.NewCertAndKey(caCert, caKey, cfg)
	if err != nil {
		return fmt.Errorf("generate client cert for %q: %w", file, err)
	}

	encodedKey, err := keyutil.MarshalPrivateKeyToPEM(newKey)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	kubeConfig, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return fmt.Errorf("load kubeconfig %q: %w", file, err)
	}

	authInfo, err := currentAuthInfo(kubeConfig)
	if err != nil {
		return fmt.Errorf("kubeconfig %q: %w", file, err)
	}

	// Switch to embedded PEM and clear file-path references so clientcmd never sees both modes set after the patch (client-certificate-data and client-certificate mutually exclusive).
	authInfo.ClientCertificate = ""
	authInfo.ClientCertificateData = pkiutil.EncodeCertificate(newCert)
	authInfo.ClientKey = ""
	authInfo.ClientKeyData = encodedKey

	if dryRun {
		return nil
	}

	if err := clientcmd.WriteToFile(*kubeConfig, path); err != nil {
		return fmt.Errorf("write kubeconfig %q: %w", file, err)
	}
	return nil
}

// RenewClientCert unconditionally re-signs the client certificate embedded in the given kubeconfig file.
// All other kubeconfig fields are preserved. CA cert and key are loaded from pkiDir.
//
// The returned error encodes the outcome:
//   - nil              — re-signed cleanly
//   - *MissingError    — kubeconfig file absent (skipped)
//   - *CAMissingError  — CA cert file absent (skipped)
//   - *CAExternalError — CA key absent / external CA (skipped)
//   - *CAExpiredError  — CA already expired (skipped)
//   - any other error  — IO/permissions/signing failure (skipped)
func RenewClientCert(file File, opts ...RenewOption) error {
	o := newRenewOptions(opts...)
	return renewClientCert(o.kubeconfigDir, o.pkiDir, file, o.dryRun)
}

// RenewClientCerts iterates the renewable kubeconfig files (or the subset chosen via WithRenewFiles) and renews each kubeconfig client certificate in turn.
// All other kubeconfig fields are preserved. CA cert and key are loaded from pkiDir.
//
// The returned error encodes the outcome:
//   - nil              — re-signed cleanly
//   - *MissingError    — kubeconfig file absent (skipped)
//   - *CAMissingError  — CA cert file absent (skipped)
//   - *CAExternalError — CA key absent / external CA (skipped)
//   - *CAExpiredError  — CA already expired (skipped)
//   - any other error  — IO/permissions/signing failure (skipped)
func RenewClientCerts(opts ...RenewOption) KubeconfigRenewReport {
	o := newRenewOptions(opts...)

	inventory := selectFiles(o.files)

	var report KubeconfigRenewReport
	for _, info := range inventory {
		path := filepath.Join(o.kubeconfigDir, string(info.File))
		report.add(info.File, path, renewClientCert(o.kubeconfigDir, o.pkiDir, info.File, o.dryRun))
	}

	return report
}

// selectFiles returns the inventory with only the given files, preserving the canonical order.
// When files is empty, returned default inventory.
func selectFiles(files []File) []fileInfo {
	full := defaultRenewableFiles()
	if len(files) == 0 {
		return full
	}

	wanted := make(map[File]struct{}, len(files))
	for _, f := range files {
		wanted[f] = struct{}{}
	}

	result := make([]fileInfo, 0, len(files))
	for _, info := range full {
		if _, ok := wanted[info.File]; ok {
			result = append(result, info)
		}
	}
	return result
}
