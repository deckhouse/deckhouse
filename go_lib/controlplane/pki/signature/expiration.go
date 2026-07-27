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

package signature

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// ActiveKeyExpiration returns the NotAfter of the active signing key certificate.
func ActiveKeyExpiration(pkiDir string) (time.Time, error) {
	privData, err := os.ReadFile(filepath.Join(pkiDir, SignaturePrivateJWK))
	if err != nil {
		return time.Time{}, fmt.Errorf("read private jwk: %w", err)
	}
	var privJWK jose.JSONWebKey
	if err := json.Unmarshal(privData, &privJWK); err != nil {
		return time.Time{}, fmt.Errorf("parse private jwk: %w", err)
	}
	jwksData, err := os.ReadFile(filepath.Join(pkiDir, SignaturePublicJWKS))
	if err != nil {
		return time.Time{}, fmt.Errorf("read jwks: %w", err)
	}
	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		return time.Time{}, fmt.Errorf("parse jwks: %w", err)
	}
	for _, key := range jwks.Keys {
		if key.KeyID == privJWK.KeyID && len(key.Certificates) > 0 {
			return key.Certificates[0].NotAfter, nil
		}
	}
	return time.Time{}, fmt.Errorf("active key %q with certificate not found in JWKS", privJWK.KeyID)
}

// SignatureFilesChecksum returns a stable hash of the on-disk signature key files, which is used to detect files changes.
func SignatureFilesChecksum(pkiDir string) (string, error) {
	h := sha256.New()
	for _, name := range []string{SignaturePrivateJWK, SignaturePublicJWKS} {
		data, err := os.ReadFile(filepath.Join(pkiDir, name))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", name, err)
		}
		_, _ = h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
