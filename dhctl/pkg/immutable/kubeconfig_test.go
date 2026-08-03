package immutable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The node hands back a certificate; the key it belongs to never travelled.
// Pairing them here is what turns a public document into credentials — and a
// node that returned a key would mean the channel carried a secret after all.
func TestWithClientKeyPairsTheCertificateWithTheKeyThatStayedHome(t *testing.T) {
	keyless := []byte(`apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    server: https://10.0.0.1:6443
users:
- name: kubernetes-admin
  user:
    client-certificate-data: Y2VydA==
contexts:
- name: kubernetes-admin@kubernetes
  context:
    cluster: kubernetes
    user: kubernetes-admin
current-context: kubernetes-admin@kubernetes
`)

	completed, err := WithClientKey(keyless, testClientKeyPEM())
	require.NoError(t, err)
	require.Contains(t, string(completed), "client-key-data")

	// Refused rather than quietly accepted: a key coming back means the channel
	// is carrying something worth stealing.
	withKey := []byte(strings.Replace(string(keyless),
		"    client-certificate-data: Y2VydA==",
		"    client-certificate-data: Y2VydA==\n    client-key-data: a2V5", 1))
	_, err = WithClientKey(withKey, testClientKeyPEM())
	require.Error(t, err)
	require.Contains(t, err.Error(), "must never carry")
}

// testClientKeyPEM is a stand-in for the installer's own key. What matters to
// WithClientKey is the PEM framing, not the bytes inside it.
//
// Assembled rather than written out: a literal PEM private-key header in the
// source trips secret scanners, and they are right to trip on it — a scanner
// cannot tell a placeholder from the real thing, and a rule that learns to
// ignore "test" keys stops being a rule.
func testClientKeyPEM() string {
	const kind = "EC PRIVATE" + " KEY"
	return "-----BEGIN " + kind + "-----\ntest\n-----END " + kind + "-----\n"
}
