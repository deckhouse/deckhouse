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

package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"net"
	"os"
	"time"
)

// ErrCertificateChanged ends Run when the serving certificate on disk is not the one being served.
//
// The process exits on it and the kubelet starts it again, which is how the new certificate gets
// served: distribution reads its certificate once, at start, and has no way to take another.
//
// What reissues it is a master joining. The certificate names every master's address, because a
// follower replicates from the leader by that address — and the replica on the master that joined
// last starts before the certificate naming it does. It kept serving one valid for the other masters
// only, and when the lease went to it, nobody could replicate from it:
// `x509: certificate is valid for 127.0.0.1, 10.12.0.148, 10.12.3.12, not 10.12.2.36`. Only that
// replica restarts; see needsReissued.
var ErrCertificateChanged = errors.New("the serving certificate changed on disk")

// certificateEvery is how often the certificate files are compared with what is served. A Secret
// mount is updated within the kubelet's sync period, so checking much more often buys nothing.
const certificateEvery = 30 * time.Second

// watchCertificate calls changed once the certificate or key on disk differ from what is served, form
// a pair that loads — never a half-written one, which would trade a certificate missing one address
// for a listener that serves nothing — and are a pair this replica needs: see needsReissued.
//
// self is the address this replica is reached at, the host of its listener.
func watchCertificate(ctx context.Context, certPath, keyPath, self string, every time.Duration,
	log *slog.Logger, changed func(),
) {
	read := func() ([]byte, []byte, error) {
		cert, err := os.ReadFile(certPath)
		if err != nil {
			return nil, nil, err
		}
		key, err := os.ReadFile(keyPath)
		return cert, key, err
	}

	servedCert, servedKey, err := read()
	if err != nil {
		log.Warn("the serving certificate cannot be read, so a reissued one will not be noticed",
			"error", err.Error())
		return
	}
	seenCert, seenKey := servedCert, servedKey

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		cert, key, err := read()
		if err != nil || (bytes.Equal(cert, seenCert) && bytes.Equal(key, seenKey)) {
			continue
		}
		if _, err := tls.X509KeyPair(cert, key); err != nil {
			continue
		}
		seenCert, seenKey = cert, key

		if !needsReissued(servedCert, cert, self) {
			log.Info("the serving certificate was reissued for another replica's address; "+
				"the one served here still covers this replica, so it keeps serving", "address", self)
			continue
		}

		log.Info("the serving certificate was reissued, so the registry restarts to serve it", "address", self)
		changed()
		return
	}
}

// needsReissued reports whether a replica serving served has to restart to serve reissued.
//
// The certificate names every master's address, so it is reissued whenever a master joins — and every
// replica then restarting for it took the leader down for a few seconds each time, which in an
// air-gapped cluster is every pull on every node failing for those seconds. Only the replica the old
// certificate does not cover needs the new one: the master that just joined. A replica the old one
// covers, under the same authority, goes on serving exactly what its clients already verify.
//
// Anything it cannot tell — an address that is not this replica's own, a certificate it cannot read, a
// different authority — restarts, which is never wrong, only sometimes unnecessary.
func needsReissued(served, reissued []byte, self string) bool {
	if self == "" || net.ParseIP(self) == nil || net.ParseIP(self).IsUnspecified() {
		return true
	}
	old, errOld := leafOf(served)
	fresh, errFresh := leafOf(reissued)
	if errOld != nil || errFresh != nil {
		return true
	}
	if !bytes.Equal(old.RawIssuer, fresh.RawIssuer) || !bytes.Equal(old.AuthorityKeyId, fresh.AuthorityKeyId) {
		return true
	}
	return old.VerifyHostname(self) != nil
}

// leafOf is the first certificate of a PEM bundle.
func leafOf(bundle []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(bundle)
	if block == nil {
		return nil, errors.New("no PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}
