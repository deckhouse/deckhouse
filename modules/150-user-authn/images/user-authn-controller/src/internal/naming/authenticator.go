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
	"regexp"
	"strings"
)

const (
	// SystemAuthenticatorInstance and UserAuthenticatorInstance are the two shared dex-authenticator
	// instances of the module, by the name of their Deployment, Service and ServiceAccount in
	// DexNamespace: the first serves the DexAuthenticator objects of the d8-* namespaces, the second
	// all the others, so that the sign-in to the platform shares no process with the applications
	// of users.
	SystemAuthenticatorInstance = "dex-authenticator-system"
	UserAuthenticatorInstance   = "dex-authenticator"

	// systemNamespacePrefix is the prefix of the namespaces only Deckhouse creates.
	systemNamespacePrefix = "d8-"

	dexAuthenticatorClientPrefix = "dex-authenticator-"
)

// AuthenticatorInstances lists the shared dex-authenticator instances.
func AuthenticatorInstances() []string {
	return []string{SystemAuthenticatorInstance, UserAuthenticatorInstance}
}

// IsSystemNamespace reports whether namespace is one only Deckhouse creates.
func IsSystemNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, systemNamespacePrefix)
}

// AuthenticatorInstance is the instance that serves the DexAuthenticator objects of a namespace.
func AuthenticatorInstance(namespace string) string {
	if IsSystemNamespace(namespace) {
		return SystemAuthenticatorInstance
	}
	return UserAuthenticatorInstance
}

// DexAuthenticatorClientID is the id of the Dex client of a DexAuthenticator. Name and namespace
// may both contain a dash, so they are joined by "@", which neither can contain.
func DexAuthenticatorClientID(name, namespace string) string {
	return dexAuthenticatorClientPrefix + name + "@" + namespace
}

// ParseDexAuthenticatorClientID returns the name and the namespace of the DexAuthenticator a
// client id belongs to, and false for the id of another client.
func ParseDexAuthenticatorClientID(id string) (string, string, bool) {
	rest, found := strings.CutPrefix(id, dexAuthenticatorClientPrefix)
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(rest, "@")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

var (
	notDNSLabelCharacters = regexp.MustCompile("[^a-z0-9-]+")
	repeatedDashes        = regexp.MustCompile("-+")
)

// SafeObjectName returns a name of at most 63 characters for an object named after a
// DexAuthenticator, whether it had to be cut, and the hash a cut name ends with. A name that fits is returned as it is. A longer
// one is cut to 57 characters and gets the first five characters of the SHA-256 of the full name,
// which the caller puts on the object as a label. The module chart named the objects by the same
// rule, so the name of an existing object does not change.
func SafeObjectName(full string) (string, bool, string) {
	if len(full) <= maxObjectNameLength {
		return full, false, ""
	}

	normalized := strings.ToLower(full)
	normalized = notDNSLabelCharacters.ReplaceAllString(normalized, "-")
	normalized = repeatedDashes.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if len(normalized) <= maxObjectNameLength {
		return normalized, false, ""
	}

	sum := sha256.Sum256([]byte(full))
	hash := hex.EncodeToString(sum[:])[:5]
	base := strings.TrimRight(normalized[:maxObjectNameLength-6], "-")
	return base + "-" + hash, true, hash
}
