/*
Copyright 2021 Flant JSC

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

package certificate

import "github.com/cloudflare/cfssl/csr"

type Option func(request *csr.CertificateRequest)

func WithKeyAlgo(algo string) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest.A = algo
	}
}

func WithKeySize(size int) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest.S = size
	}
}

func WithCAExpiry(expiry string) Option {
	return func(request *csr.CertificateRequest) {
		request.CA.Expiry = expiry
	}
}

func WithCAConfig(caConfig *csr.CAConfig) Option {
	return func(request *csr.CertificateRequest) {
		request.CA = caConfig
	}
}

func WithKeyRequest(keyRequest *csr.KeyRequest) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest = keyRequest
	}
}

func WithGroups(groups ...string) Option {
	return func(request *csr.CertificateRequest) {
		for _, group := range groups {
			request.Names = append(request.Names, csr.Name{O: group})
		}
	}
}

func WithSANs(sans ...string) Option {
	return func(request *csr.CertificateRequest) {
		request.Hosts = append(request.Hosts, sans...)
	}
}

// WithCSRKeyRequest redeclare basic(ecdsa 2048) key alg and size
func WithCSRKeyRequest(keyRequest *csr.KeyRequest) Option {
	return func(request *csr.CertificateRequest) {
		request.KeyRequest = keyRequest
	}
}

func WithNames(names ...csr.Name) Option {
	return func(request *csr.CertificateRequest) {
		request.Names = append(request.Names, names...)
	}
}
