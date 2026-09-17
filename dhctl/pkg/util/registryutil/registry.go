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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func NewRegistryClient(ctx context.Context, scheme, ca string) (*http.Client, error) {
	transport, err := NewRegistryTransport(ctx, scheme, ca)
	if err != nil {
		return nil, fmt.Errorf("creating registry transport: %w", err)
	}

	return &http.Client{Transport: transport}, nil
}

// NameOptions maps the registry scheme to go-containerregistry reference options.
// Without name.Insecure the library never tries HTTP; it only permits it, so the
// transport paired with these must still verify certificates.
func NameOptions(scheme string) []name.Option {
	if constant.ToScheme(scheme) == constant.SchemeHTTP {
		return []name.Option{name.Insecure}
	}

	return nil
}

func NewRegistryTransport(ctx context.Context, scheme, ca string) (*http.Transport, error) {
	if strings.EqualFold(scheme, "http") {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		return transport, nil
	}

	return NewCARegistryTransport(ctx, ca)
}

// NewCARegistryTransport verifies certificates, trusting ca on top of the system pool.
func NewCARegistryTransport(ctx context.Context, ca string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if ca == "" {
		return transport, nil
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot get system CAs pool, falling back to custom CA pool only: %v", err))
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
