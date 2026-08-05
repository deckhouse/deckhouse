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

package bashible

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// localBundleStepsDir mirrors stepsDir in dhctl/pkg/template/bundle.go
// ("/var/lib/bashible/bundle_steps"), relative to a rendered bundle's root
// (templateController.TmpDir).
const localBundleStepsDir = "var/lib/bashible/bundle_steps"

// VerifyStepsStatus compares previously recorded bashible bundle step
// checksums (name -> sha256, as saved by dhctl from an earlier bootstrap
// attempt) against the freshly rendered local bundle in bundleDir. It
// returns an error naming any step whose content changed since it last
// completed successfully.
//
// A mismatch means the cluster config or dhctl/candi version changed between
// bootstrap attempts. Silently re-running such a step would apply it against
// a node that may already reflect its OLD version and later steps may depend
// on that — unsafe to do automatically, so bootstrap should stop rather than
// resume.
func VerifyStepsStatus(bundleDir string, saved map[string]string) error {
	names := make([]string, 0, len(saved))
	for name := range saved {
		names = append(names, name)
	}
	sort.Strings(names)

	var changed []string

	for _, name := range names {
		path := filepath.Join(bundleDir, localBundleStepsDir, name)

		content, err := os.ReadFile(path)
		if err != nil {
			changed = append(changed, fmt.Sprintf("%s (%v)", name, err))
			continue
		}

		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])
		if checksum != saved[name] {
			changed = append(changed, name)
		}
	}

	if len(changed) == 0 {
		return nil
	}

	return fmt.Errorf(stepsChangedErrorTemplate, strings.Join(changed, ", "))
}

const stepsChangedErrorTemplate = `Bootstrap cannot safely resume: the following bashible bundle steps already completed in a previous bootstrap attempt, but their content has changed since then: %s.

Resuming would re-run them against a node that already applied their old version, which is unsafe.

Please run "dhctl bootstrap-phase abort" to clean up, then start a fresh bootstrap.`
