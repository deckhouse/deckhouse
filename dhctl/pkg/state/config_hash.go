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

package state

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

func ConfigHash(paths []string) string {
	const hashLen = 8

	resolvedPaths := fs.RevealWildcardPaths(paths)
	sort.Strings(resolvedPaths)

	h := sha256.New()
	for _, path := range resolvedPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			log.WarnF("cannot read config file %s for preflight cache hash: %v\n", path, err)
			continue
		}
		if _, err := h.Write(data); err != nil {
			log.WarnF("cannot hash config file %s for preflight cache hash: %v\n", path, err)
		}
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if len(hash) > hashLen {
		return hash[:hashLen]
	}
	return hash
}
