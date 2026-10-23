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

package k8s

import (
	"encoding/base32"
	"hash/fnv"
	"strings"
)

// The names of UserAccount objects, as user-authn-controller gives them (internal/naming of the
// controller). user-api may only get a UserAccount by its name, never list them.

const (
	maxObjectNameLength = 63

	// localConnectorID is the connector of the local users of Dex.
	localConnectorID = "local"
)

// dexEncoding is the lowercase base32 alphabet Dex names its objects with.
var dexEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

// LocalAccountName is the name of the UserAccount of a local user, from the email of its Password
// object, or the username when the Password has no email.
func LocalAccountName(email, username string) string {
	input := email
	if input == "" {
		input = username
	}
	// Dex idToName: fnv.New64().Sum(b) appends the checksum of nothing to b.
	hash := strings.TrimRight(dexEncoding.EncodeToString(fnv.New64().Sum([]byte(strings.ToLower(input)))), "=")
	return fitObjectName(localConnectorID+"-"+hash, hash)
}

// ExternalAccountName is the name of the UserAccount of the pair (userID, connectorID) of an
// external provider.
func ExternalAccountName(connectorID, userID string) string {
	h := fnv.New64()
	// hash.Hash.Write never returns an error.
	_, _ = h.Write([]byte(userID))
	_, _ = h.Write([]byte(connectorID))
	hash := strings.TrimRight(dexEncoding.EncodeToString(h.Sum(nil)), "=")
	return fitObjectName(connectorID+"-"+hash, hash)
}

func fitObjectName(full, hash string) string {
	if len(full) > maxObjectNameLength {
		return hash
	}
	return full
}
