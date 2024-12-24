/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package transport

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func NewHttpRoundTripper(systemCertPool bool, caPath ...string) (http.RoundTripper, error) {
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
