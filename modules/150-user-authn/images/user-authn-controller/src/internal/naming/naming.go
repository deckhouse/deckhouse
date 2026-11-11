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
	"encoding/base32"
	"hash/fnv"
	"strings"
)

const (
	// DexNamespace is where Dex stores Password, OfflineSessions, and RefreshToken.
	DexNamespace = "d8-user-authn"

	// LocalConnectorID is Dex's connector id for password-based local users.
	LocalConnectorID = "local"
)

const maxObjectNameLength = 63

// maxDexObjectNameLength is the longest name of a Dex storage object: a Kubernetes object name,
// a DNS subdomain.
const maxDexObjectNameLength = 253

// MaxLocalEmailBytes is the longest email of a local user, in bytes. Dex names the Password of
// the user ToFnvLikeDex of the email: the unpadded base32 of the email and the 8 bytes of its
// checksum, 8/5 as long. (150+8)*8/5 rounds up to 253, the longest object name; 151 bytes make 255.
const MaxLocalEmailBytes = 150

// dexEncoding matches Dex's lowercase base32 alphabet used for Kubernetes object names.
var dexEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

// ToFnvLikeDex is Dex idToName: fnv.New64().Sum([]byte(s)) appends the empty-hash
// checksum to s, then base32-encodes. That differs from Write+Sum(nil); use
// OfflineTokenName for OfflineSessions.
func ToFnvLikeDex(s string) string {
	return strings.TrimRight(dexEncoding.EncodeToString(fnv.New64().Sum([]byte(s))), "=")
}

// PasswordNameFits reports whether the name of the Password Dex keeps for a local user with this
// email fits an object name. Dex lowercases the email before it names the Password.
func PasswordNameFits(email string) bool {
	return len(ToFnvLikeDex(strings.ToLower(email))) <= maxDexObjectNameLength
}

// OfflineTokenName matches Dex offlineTokenName: hash.Write(userID); hash.Write(connID); Sum(nil).
func OfflineTokenName(userID, connID string) string {
	h := fnv.New64()
	// hash.Hash.Write never returns a non-nil error.
	_, _ = h.Write([]byte(userID))
	_, _ = h.Write([]byte(connID))
	return strings.TrimRight(dexEncoding.EncodeToString(h.Sum(nil)), "=")
}

// LocalName is local-<ToFnvLikeDex(lower(email))>, or only the hash if the name exceeds 63 characters.
func LocalName(email string) string {
	hash := ToFnvLikeDex(strings.ToLower(email))
	return fitObjectName(LocalConnectorID+"-"+hash, hash)
}

// ExternalName is <connID>-<OfflineTokenName(userID, connID)>, or only the hash if the name exceeds 63 characters.
func ExternalName(connID, userID string) string {
	hash := OfflineTokenName(userID, connID)
	return fitObjectName(connID+"-"+hash, hash)
}

func fitObjectName(full, hash string) string {
	if len(full) > maxObjectNameLength {
		return hash
	}
	return full
}
