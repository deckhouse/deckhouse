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

package provider

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// idTokenSubject reads the sub and d8_gen claims from the payload of an ID token; d8_gen is 0 when
// absent. The signature is not checked: the token comes straight from the token endpoint of Dex,
// over the connection that has just validated the password, and it is trusted as much as the groups
// that come over the same connection.
func idTokenSubject(rawIDToken string) (string, uint64, error) {
	if rawIDToken == "" {
		return "", 0, errors.New("the token response has no ID token")
	}

	parts := strings.Split(rawIDToken, ".")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("the ID token has %d parts, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("decode the ID token payload: %w", err)
	}

	var claims struct {
		Subject    string `json:"sub"`
		Generation uint64 `json:"d8_gen"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", 0, fmt.Errorf("parse the ID token claims: %w", err)
	}
	if claims.Subject == "" {
		return "", 0, errors.New("the ID token has no sub claim")
	}

	return claims.Subject, claims.Generation, nil
}
