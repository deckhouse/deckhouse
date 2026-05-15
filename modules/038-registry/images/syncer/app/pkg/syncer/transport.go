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

package syncer

import (
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func newHTTPRoundTripper(systemCertPool bool, caCerts ...string) (http.RoundTripper, error) {
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

	for _, caCert := range caCerts {
		if !certPool.AppendCertsFromPEM([]byte(caCert)) {
			return nil, fmt.Errorf("load CA file %v: no valid certificates found", caCert)
		}
	}

	ret.TLSClientConfig.RootCAs = certPool

	return ret, nil
}
