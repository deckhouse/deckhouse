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

// Package licensing implements the pure-Go core of the Deckhouse licensing MVP:
// compact JWS handling, registration request building, license package
// verification and computation of the resulting license policy.
//
// It deliberately depends on the standard library only, so that it can be
// reused by hooks, by the controller and by command line tooling without
// dragging Kubernetes machinery along.
package licensing

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Token types and issuer defined by the registration specification.
const (
	// TypRegistration is the "typ" header value of a cluster registration request.
	TypRegistration = "dkp-cluster-registration+jwt"
	// TypLicense is the "typ" header value of a license package issued by the license server.
	TypLicense = "dkp-license+jwt"
	// Issuer is the only accepted "iss" claim of a license package.
	Issuer = "license.deckhouse.io"
	// SchemaVersion is the highest payload schema version this build understands.
	SchemaVersion = 1

	algEdDSA = "EdDSA"
)

// Sentinel errors returned by Parse and ParsePackage. Use errors.Is to classify.
var (
	// ErrMalformed means the token is not a well formed compact JWS, or its
	// payload does not satisfy the package level schema rules.
	ErrMalformed = errors.New("licensing: malformed token")
	// ErrUnsupportedAlg means the "alg" header is not EdDSA. The value from the
	// header is never used to select verification logic (RFC 8725).
	ErrUnsupportedAlg = errors.New("licensing: unsupported algorithm")
	// ErrBadSignature means no known key verifies the signature.
	ErrBadSignature = errors.New("licensing: bad signature")
	// ErrWrongType means the "typ" header does not match the expected token type.
	ErrWrongType = errors.New("licensing: wrong token type")
	// ErrUnsupportedVersion means the package schema is newer than this build
	// understands. It wraps ErrMalformed, so existing callers keep classifying it
	// as a malformed package; it is told apart only where the difference matters,
	// which is retirability: this is the one package level failure that heals on
	// its own, with a Deckhouse upgrade (spec 8.6).
	ErrUnsupportedVersion = fmt.Errorf("%w: unsupported schema version", ErrMalformed)
)

// Characters that survive copy-paste through browsers, chats and PDF viewers.
const (
	zeroWidthSpace = 0x200B
	byteOrderMark  = 0xFEFF
)

// Normalize strips every whitespace character (including NBSP U+00A0) and the
// zero width characters U+200B and U+FEFF from s. Tokens are transferred by
// hand, so line wraps and invisible characters are expected, not exceptional.
func Normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == zeroWidthSpace || r == byteOrderMark || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// Thumbprint returns the RFC 7638 JWK thumbprint of an Ed25519 public key:
// SHA-256 over the canonical JWK with its three required members in
// lexicographic order, base64url encoded without padding.
func Thumbprint(pub ed25519.PublicKey) string {
	c := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`,
		base64.RawURLEncoding.EncodeToString(pub))
	s := sha256.Sum256([]byte(c))
	return base64.RawURLEncoding.EncodeToString(s[:])
}

// marshalCompact serializes v the way the license server (Ruby) does: no HTML
// escaping of <, > and &, and no trailing newline.
func marshalCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Sign builds a bare compact JWS over payload. The "alg" header is always set
// to EdDSA; the caller supplied header is copied, never mutated.
func Sign(header map[string]any, payload any, priv ed25519.PrivateKey) (string, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return "", errors.New("licensing: invalid Ed25519 private key")
	}

	h := make(map[string]any, len(header)+1)
	for k, v := range header {
		h[k] = v
	}
	h["alg"] = algEdDSA

	hb, err := marshalCompact(h)
	if err != nil {
		return "", fmt.Errorf("licensing: marshal header: %w", err)
	}
	pb, err := marshalCompact(payload)
	if err != nil {
		return "", fmt.Errorf("licensing: marshal payload: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)
	sig := ed25519.Sign(priv, []byte(signingInput))

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Token is a parsed compact JWS. The signing input is kept as received: the
// signature is verified over the original bytes, never over a re-serialization
// of the parsed structures.
type Token struct {
	Header  map[string]any
	Payload []byte

	signingInput []byte
	signature    []byte
}

// Parse normalizes s and decodes it as a bare compact JWS. It hard-requires
// alg == EdDSA and never dispatches on the header value.
func Parse(s string) (*Token, error) {
	parts := strings.Split(Normalize(s), ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("%w: expected three non-empty segments", ErrMalformed)
	}

	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header is not base64url: %s", ErrMalformed, err)
	}
	var header map[string]any
	if err := json.Unmarshal(rawHeader, &header); err != nil {
		return nil, fmt.Errorf("%w: header is not JSON: %s", ErrMalformed, err)
	}
	alg, _ := header["alg"].(string)
	if alg != algEdDSA {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAlg, alg)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: payload is not base64url: %s", ErrMalformed, err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: signature is not base64url: %s", ErrMalformed, err)
	}

	return &Token{
		Header:       header,
		Payload:      payload,
		signingInput: []byte(parts[0] + "." + parts[1]),
		signature:    signature,
	}, nil
}

// Verify reports whether the token signature is valid for pub.
func (t *Token) Verify(pub ed25519.PublicKey) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pub, t.signingInput, t.signature)
}
