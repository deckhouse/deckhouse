// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jwt

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"time"

	"github.com/square/go-jose/v3"
)

type payloadMap map[string]interface{}

func GenerateJWT(privKeyPEMBytes []byte, claims map[string]string, ttl time.Duration) (string, error) {
	keyBlock, _ := pem.Decode(privKeyPEMBytes)
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return "", err
	}

	signerKey := jose.SigningKey{Algorithm: jose.EdDSA, Key: key}
	var signerOpts = jose.SignerOptions{}
	tokenSigner, err := jose.NewSigner(signerKey, &signerOpts)
	if err != nil {
		return "", err
	}

	tokenClaims := payloadMap{}
	for key, value := range claims {
		tokenClaims[key] = value
	}
	tokenClaims["nbf"] = time.Now().UTC().Unix()
	tokenClaims["exp"] = time.Now().Add(ttl).UTC().Unix()

	tokenClaimsBytes, err := json.Marshal(tokenClaims)
	if err != nil {
		return "", err
	}

	tokenSignature, err := tokenSigner.Sign(tokenClaimsBytes)
	if err != nil {
		return "", err
	}

	tokenString, err := tokenSignature.CompactSerialize()
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
