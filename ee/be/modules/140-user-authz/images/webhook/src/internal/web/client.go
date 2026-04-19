/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package web

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

func NewClient() (*http.Client, error) {
	clientTLSCert, err := tls.LoadX509KeyPair(sslListenCert, sslListenKey)
	if err != nil {
		return nil, fmt.Errorf("error loading certificate and key file: %w", err)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{clientTLSCert},
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{Transport: tr}
	return client, nil
}
