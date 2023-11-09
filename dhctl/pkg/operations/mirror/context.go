// Copyright 2023 Flant JSC
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

package mirror

import (
	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
)

// Context hold data related to pending registry mirroring operation.
type Context struct {
	Insecure        bool // --insecure
	SkipGOSTDigests bool // --skip-gost-digests

	RegistryAuth authn.Authenticator // --registry-login + --registry-password (can be nil in this case) or --license depending on the operation requested
	RegistryHost string              // --registry
	RegistryRepo string

	TarBundlePath      string // --images
	UnpackedImagesPath string
	ValidationMode     ValidationMode  // --validation, hidden flag
	MinVersion         *semver.Version // --min-version
}
