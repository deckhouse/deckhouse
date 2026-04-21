// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registryutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func NewRegistryClient(scheme, ca string) (*http.Client, error) {
	transport, err := NewRegistryTransport(scheme, ca)
	if err != nil {
		return nil, fmt.Errorf("creating registry transport: %w", err)
	}

	return &http.Client{Transport: transport}, nil
}

func NewRegistryTransport(scheme, ca string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if strings.EqualFold(scheme, "http") {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		return transport, nil
	}

	if ca == "" {
		return transport, nil
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.WarnF("Cannot get system CAs pool, fallback to custom CA pool only: %v\n", err)
		certPool = x509.NewCertPool()
	}
	if certPool == nil {
		certPool = x509.NewCertPool()
	}

	if ok := certPool.AppendCertsFromPEM([]byte(ca)); !ok {
		return nil, fmt.Errorf("invalid cert in CA PEM")
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: certPool}
	return transport, nil
}
