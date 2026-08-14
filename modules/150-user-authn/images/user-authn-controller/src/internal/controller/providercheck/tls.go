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

package providercheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

func checkCABundle(result *dexProviderCheckResult, stepName, rootCAData string) {
	if rootCAData == "" {
		result.skip(stepName, "No custom CA provided; using the system trust store")
		return
	}

	notAfter, err := earliestCertExpiry([]byte(rootCAData))
	if err != nil {
		result.fail(stepName, "rootCAData is invalid: %v", err)
		return
	}
	reportExpiry(result, stepName, "rootCAData certificate", notAfter)
}

func earliestCertExpiry(pemData []byte) (time.Time, error) {
	rest := pemData
	var earliest time.Time
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse certificate: %w", err)
		}
		if !found || cert.NotAfter.Before(earliest) {
			earliest = cert.NotAfter
			found = true
		}
	}
	if !found {
		return time.Time{}, errors.New("no certificates found")
	}
	return earliest, nil
}

func checkTLSCertificate(ctx context.Context, result *dexProviderCheckResult, stepName, rawURL, rootCAData string, insecureSkipVerify bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		result.skip(stepName, "endpoint is not HTTPS; certificate check skipped")
		return
	}

	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	tlsConfig, err := buildTLSConfig(rootCAData, parsed.Hostname(), insecureSkipVerify)
	if err != nil {
		result.fail(stepName, "cannot build TLS config: %v", err)
		return
	}

	dialer := &net.Dialer{Timeout: httpTimeout}
	addr := net.JoinHostPort(parsed.Hostname(), port)
	nconn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.failUnreachable(stepName, err, "cannot establish TLS connection: %v", err)
		return
	}

	tlsConn := tls.Client(nconn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		result.failUnreachable(stepName, err, "cannot establish TLS connection: %v", err)
		return
	}
	defer func() { _ = tlsConn.Close() }()

	certs := tlsConn.ConnectionState().PeerCertificates
	if insecureSkipVerify {
		if len(certs) == 0 {
			result.warn(stepName, "TLS certificate verification is disabled (insecureSkipVerify); the server certificate is not validated")
			return
		}
		result.warn(stepName,
			"TLS certificate verification is disabled (insecureSkipVerify); the server certificate (valid until %s) is not validated",
			certs[0].NotAfter.UTC().Format(time.RFC3339))
		return
	}
	reportLeafExpiry(result, stepName, certs)
}

func reportLeafExpiry(result *dexProviderCheckResult, stepName string, certs []*x509.Certificate) {
	if len(certs) == 0 {
		result.fail(stepName, "server did not present a TLS certificate")
		return
	}
	reportExpiry(result, stepName, "server TLS certificate", certs[0].NotAfter)
}

func reportExpiry(result *dexProviderCheckResult, stepName, subject string, notAfter time.Time) {
	now := result.now
	if now.IsZero() {
		now = time.Now()
	}
	remaining := notAfter.Sub(now)
	expiry := notAfter.UTC().Format(time.RFC3339)
	switch {
	case remaining <= 0:
		result.fail(stepName, "%s expired on %s", subject, expiry)
	case remaining <= certExpiryWarnWindow:
		result.warn(stepName, "%s expires soon, on %s", subject, expiry)
	default:
		result.succeed(stepName, "%s is valid until %s", subject, expiry)
	}
}

func buildTLSConfig(rootCAData, serverName string, insecureSkipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify,
		ServerName:         serverName,
	}
	if insecureSkipVerify {
		return tlsConfig, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system CA pool: %w", err)
	}
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if rootCAData != "" && !pool.AppendCertsFromPEM([]byte(rootCAData)) {
		return nil, errors.New("append rootCAData: no certificates found")
	}
	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}
