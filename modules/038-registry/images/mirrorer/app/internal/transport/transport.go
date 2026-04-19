/*
Copyright 2025 Flant JSC

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
package transport

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func NewHTTPRoundTripper(systemCertPool bool, caPath ...string) (http.RoundTripper, error) {
	ret := remote.DefaultTransport.(*http.Transport).Clone()

	var (
		certPool *x509.CertPool
		err      error
	)

	if systemCertPool {
		certPool, err = x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("cannot get system CAs pool: %w", err)
		}
	} else {
		certPool = x509.NewCertPool()
	}

	for _, file := range caPath {
		pem, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("load CA file %v error: %w", file, err)
		}

		certPool.AppendCertsFromPEM(pem)
	}

	ret.TLSClientConfig.RootCAs = certPool

	return ret, nil
}
