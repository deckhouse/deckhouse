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

package bashible

import (
	"crypto/sha256"
	"encoding/hex"
)

// InstallerMarkAnnotation is the annotation the installer puts on the `registry-bashible-config` it
// writes, set to the InstallerMark of the document it wrote.
//
// It is how the registry module tells that document from one the v1 implementation left. The two
// cannot be told apart by their contents: an installation with no registry settings, or with a
// registry module set to manage nothing, writes `mode: Direct` with no agent, exactly as a v1
// `Direct` cluster has it, and an installation from a bundle writes `mode: Local` with the node
// services' endpoints, exactly as a v1 `Local` cluster has it. Neither the v1 implementation nor any
// installer of a release that had one ever set this annotation (checked on 1.75 to 1.77), so its
// presence alone says the document was written by an installer of a release without it.
//
// Set to the digest of the document rather than to a fixed value, so that it speaks only for the
// document it was written with: a document that something has rewritten since — the v1
// implementation of an older release a newer installer installed, or anyone editing it by hand — no
// longer matches its mark and is read for what it says again.
const InstallerMarkAnnotation = "registry.deckhouse.io/installer-config-sha256"

// InstallerMark is the value of InstallerMarkAnnotation for a document: the hex SHA-256 of its bytes
// exactly as they are stored under the `config` key.
func InstallerMark(document []byte) string {
	sum := sha256.Sum256(document)
	return hex.EncodeToString(sum[:])
}

// WrittenByInstaller reports that a document is still exactly the one the installer wrote and marked.
func WrittenByInstaller(annotations map[string]string, document []byte) bool {
	mark, ok := annotations[InstallerMarkAnnotation]
	return ok && len(document) > 0 && mark == InstallerMark(document)
}
