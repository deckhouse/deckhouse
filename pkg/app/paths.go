// Copyright 2026 Flant JSC
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

package app

// Filesystem paths baked into the deckhouse image layout.
const (
	// PathVersion is the file holding the deckhouse version.
	PathVersion = "/deckhouse/version"
	// PathCRDs is the glob for deckhouse-controller CRD manifests.
	PathCRDs = "/deckhouse/deckhouse-controller/crds/*.yaml"
	// PathEdition is the file holding the deckhouse edition.
	PathEdition = "/deckhouse/edition"
	// PathEmbeddedModules is the directory with modules shipped in the image.
	PathEmbeddedModules = "/deckhouse/modules"
)

// Chroot mount sources for shell hooks and enabled scripts.
const (
	PathPythonLib      = "/deckhouse/python_lib"
	PathCandi          = "/deckhouse/candi"
	PathHelmLib        = "/deckhouse/helm_lib"
	PathShellLib       = "/deckhouse/shell_lib"
	PathShellOperator  = "/deckhouse/shell-operator"
	PathShellLibScript = "/deckhouse/shell_lib.sh"
)
