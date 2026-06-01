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
	"encoding/pem"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/util/pkiutil"
)

type ExpirationOption func(*expirationOptions)

type expirationOptions struct {
	kubeconfigDir    string
	files            []File
	ignoreReadErrors bool
}

type ClientCertificateExpiration struct {
	File     File
	Path     string
	NotAfter time.Time
}

var defaultExpirationFiles = []File{
	Admin,
	ControllerManager,
	Scheduler,
	SuperAdmin,
}

// WithKubeconfigDir overrides the directory used by ListClientCertificateExpirations.
func WithKubeconfigDir(dir string) ExpirationOption {
	return func(o *expirationOptions) {
		o.kubeconfigDir = dir
	}
}

// WithFiles restricts ListClientCertificateExpirations to the provided kubeconfig files.
func WithFiles(files ...File) ExpirationOption {
	return func(o *expirationOptions) {
		o.files = append(o.files, files...)
	}
}

// WithIgnoreReadErrors enables partial success and returns read failures as errors.Join(...).
func WithIgnoreReadErrors() ExpirationOption {
	return func(o *expirationOptions) {
		o.ignoreReadErrors = true
	}
}

func ListClientCertificateExpirations(opts ...ExpirationOption) ([]ClientCertificateExpiration, error) {
	options := newExpirationOptions(opts...)
	files := expirationFiles(options)

	result := make([]ClientCertificateExpiration, 0, len(files))
	var errs []error

	for _, file := range files {
		path := filepath.Join(options.kubeconfigDir, string(file))
		expiration, err := clientCertificateExpiration(path)
		if err != nil {
			if !options.ignoreReadErrors {
				return nil, err
			}

			errs = append(errs, err)
			continue
		}

		result = append(result, expiration)
	}

	return result, errors.Join(errs...)
}

func GetClientCertificateExpiration(path string) (ClientCertificateExpiration, error) {
	return clientCertificateExpiration(path)
}

func clientCertificateExpiration(path string) (ClientCertificateExpiration, error) {
	cert, err := loadClientCertificate(path)
	if err != nil {
		return ClientCertificateExpiration{}, err
	}

	return ClientCertificateExpiration{
		File:     canonicalFile(path),
		Path:     filepath.Clean(path),
		NotAfter: cert.NotAfter,
	}, nil
}

func loadClientCertificate(path string) (*x509.Certificate, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig %q: %w", path, err)
	}

	authInfo, err := currentAuthInfo(config)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig %q: %w", path, err)
	}

	if len(authInfo.ClientCertificateData) > 0 {
		block, _ := pem.Decode(authInfo.ClientCertificateData)
		if block == nil || len(block.Bytes) == 0 {
			return nil, fmt.Errorf("failed to read kubeconfig %q: client-certificate-data is not valid PEM", path)
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read kubeconfig %q: client-certificate-data is not a valid certificate: %w", path, err)
		}

		return cert, nil
	}

	if authInfo.ClientCertificate != "" {
		clientCertPath := authInfo.ClientCertificate
		if !filepath.IsAbs(clientCertPath) {
			clientCertPath = filepath.Join(filepath.Dir(path), clientCertPath)
		}

		cert, err := pkiutil.LoadCert(clientCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read kubeconfig %q: %w", path, err)
		}

		return cert, nil
	}

	return nil, fmt.Errorf("failed to read kubeconfig %q: client certificate is not configured", path)
}

func newExpirationOptions(opts ...ExpirationOption) *expirationOptions {
	options := &expirationOptions{
		kubeconfigDir: DefaultOutDir,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}

func expirationFiles(options *expirationOptions) []File {
	selected := make(map[File]struct{})
	files := options.files
	if len(files) == 0 {
		files = defaultExpirationFiles
	}

	for _, file := range files {
		selected[file] = struct{}{}
	}

	result := make([]File, 0, len(selected))
	for file := range selected {
		result = append(result, file)
	}

	sort.Slice(result, func(i, j int) bool {
		return string(result[i]) < string(result[j])
	})

	return result
}

func canonicalFile(path string) File {
	base := File(filepath.Base(filepath.Clean(path)))

	for _, file := range append(defaultExpirationFiles, Kubelet) {
		if base == file {
			return file
		}
	}

	return base
}

func currentAuthInfo(config *clientcmdapi.Config) (*clientcmdapi.AuthInfo, error) {
	currentContext, ok := config.Contexts[config.CurrentContext]
	if !ok {
		return nil, fmt.Errorf("current context %q not found", config.CurrentContext)
	}

	authInfo, ok := config.AuthInfos[currentContext.AuthInfo]
	if !ok {
		return nil, fmt.Errorf("auth info %q not found", currentContext.AuthInfo)
	}

	return authInfo, nil
}
