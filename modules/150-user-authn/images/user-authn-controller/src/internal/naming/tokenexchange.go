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

package naming

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

const (
	// tokenExchangeConnectorPrefix starts every connector ID of a TokenExchangeProvider, so none
	// collides with the connectors of the Dex config, which win over the storage on the same ID.
	tokenExchangeConnectorPrefix = "tep-"
	// tokenExchangeUIDLength is how much of the UID the connector ID carries: a provider deleted and
	// created again under the same name gets another connector, which no revocation of the old one
	// names.
	tokenExchangeUIDLength = 8

	// MaxTokenExchangeConnectorIDs bounds the IDs a TokenExchangeProvider may take; revocations
	// expire, so a provider never needs more than a few.
	MaxTokenExchangeConnectorIDs = 100
)

// TokenExchangeConnectorID is the ID of the nth Dex connector of the TokenExchangeProvider name with
// the UID uid. A connector that was revoked (the provider lost its names to an earlier one) is never
// used again, since its revocation refuses every token issued through it, so the next one gets the
// next ID. The ID keeps the length of the first one, which fits label values.
//
// The tokenexchangeprovider controller names the connectors with it and the dexclient controller
// allows them in the clients a provider lists.
func TokenExchangeConnectorID(name, uid string, n int) string {
	if n == 0 {
		uid = strings.ReplaceAll(uid, "-", "")
		if len(uid) > tokenExchangeUIDLength {
			uid = uid[:tokenExchangeUIDLength]
		}
		return tokenExchangeConnectorPrefix + name + "-" + uid
	}
	sum := sha256.Sum256([]byte(uid + "/" + strconv.Itoa(n)))
	return tokenExchangeConnectorPrefix + name + "-" + hex.EncodeToString(sum[:])[:tokenExchangeUIDLength]
}

// IsTokenExchangeConnectorID reports whether id is one of the connector IDs of the
// TokenExchangeProvider name with the UID uid.
func IsTokenExchangeConnectorID(name, uid, id string) bool {
	for n := range MaxTokenExchangeConnectorIDs {
		if TokenExchangeConnectorID(name, uid, n) == id {
			return true
		}
	}
	return false
}
