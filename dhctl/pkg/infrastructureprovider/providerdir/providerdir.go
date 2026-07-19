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

package providerdir

import (
	"path/filepath"
	"strings"
)

// ProviderDir returns the stable per-provider root under root
// (<root>/<provider>). Once a bundle is unpacked it is a symlink to the
// current ProviderDigestDir.
// PlanRulesFilename is the terraform plan-rules file of a single-provider
// bundle. It lives next to terraform_versions.yml and describes exactly that
// file's provider, so the two must always be written and removed as a pair:
// a plan_rules.yml next to a multi-provider versions file is rejected by the
// settings loader.
const PlanRulesFilename = "plan_rules.yml"

func ProviderDir(root, provider string) string {
	return filepath.Join(root, strings.ToLower(provider))
}

// ProviderDigestDir returns the digest-pinned unpack directory for a provider
// bundle (<root>/<provider>@<digest>).
func ProviderDigestDir(root, provider, digest string) string {
	return filepath.Join(root, strings.ToLower(provider)+"@"+digest)
}

// ValidatorPath returns the expected location of the provider's external
// validator binary inside the unpacked bundle.
func ValidatorPath(root, provider string) string {
	return filepath.Join(ProviderDir(root, provider), "validator")
}
