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

package fill

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ClientCertificate is a key pair, by file, that a client presents to a listener that asks for one.
//
// What it is for is this replica's own write endpoint, which refuses a handshake without a client
// certificate from the module's authority: the fill, the replication and the garbage collection all
// write through it. Kept as files rather than as a loaded pair because the files are a Secret mount
// the hook reissues when a master joins, and a pair loaded once would be presented long after it was
// replaced.
type ClientCertificate struct {
	CertFile string
	KeyFile  string
}

// Present makes transport offer the pair to any listener that asks for a certificate its issuer
// signed, and to nothing else. A nil certificate leaves the transport as it is, so a caller that
// may or may not have one does not need to ask.
//
// The one call a client of this replica's registry has to make to be admitted to the write endpoint,
// whatever else builds its transport — the registry options below and anything that has to speak
// plain HTTP to the same listener alike.
//
// Offered rather than always sent, which is what keeps it harmless on every other listener: a
// server that does not ask is never told, and one that asks for certificates of another authority
// gets none, as Go's own selection would decide for a pair given in Certificates. The serving
// listener this replica and its followers pull through does not ask.
func (c *ClientCertificate) Present(transport *http.Transport) {
	if c == nil {
		return
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.GetClientCertificate = c.load
}

// load reads the pair when a listener asks for it.
//
// A pair that cannot be read fails the handshake with the reason, rather than going out without a
// certificate and coming back as a refusal that does not say why.
func (c *ClientCertificate) load(request *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	pair, err := loadPair(c.CertFile, c.KeyFile, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("loading the client certificate %s: %w", c.CertFile, err)
	}
	if request.SupportsCertificate(&pair) != nil {
		// Not asked for by this listener. An empty certificate is "none", which is what the
		// standard library sends when nothing in Certificates fits.
		return &tls.Certificate{}, nil
	}
	return &pair, nil
}

// pairReadAttempts is how many times loadPair reads the two files before it gives up on a pair that
// does not match.
const pairReadAttempts = 3

// loadPair reads the certificate and the key, and takes them only if the key is the certificate's.
//
// Two reads of a Secret mount are not one: kubelet replaces the whole pair at once, by swapping the
// directory both files are links into, and a swap between the two reads gives a certificate of one
// issue and a key of the other. That pair is refused, so both are read again; the swap is over by
// then. A mismatch that outlasts the attempts is a mount that holds one, and is reported as such.
func loadPair(certFile, keyFile string, read func(string) ([]byte, error)) (tls.Certificate, error) {
	var err error
	for range pairReadAttempts {
		var certPEM, keyPEM []byte
		if certPEM, err = read(certFile); err != nil {
			continue
		}
		if keyPEM, err = read(keyFile); err != nil {
			continue
		}
		var pair tls.Certificate
		if pair, err = tls.X509KeyPair(certPEM, keyPEM); err == nil {
			return pair, nil
		}
	}
	return tls.Certificate{}, err
}

// ClientTransport is the transport for a registry whose listener may require a client certificate:
// trusting certificateAuthority and presenting client, the way ClientRegistryOptions does. For
// anything that speaks plain HTTP to such a listener rather than through the registry options — a
// probe, a health check — so that it is admitted exactly when the copy is. A nil client presents
// nothing.
func ClientTransport(certificateAuthority string, client *ClientCertificate) (*http.Transport, error) {
	transport, err := Transport(certificateAuthority)
	if err != nil {
		return nil, err
	}
	client.Present(transport)
	return transport, nil
}

// ClientRegistryOptions is RegistryOptions that also presents client, for a registry whose listener
// requires a client certificate. A nil client gives exactly RegistryOptions.
func ClientRegistryOptions(
	certificateAuthority, username, password string, client *ClientCertificate,
) ([]remote.Option, error) {
	return registryOptions(certificateAuthority, username, password, client, nil)
}
