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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

func ConfigHash(ctx context.Context, paths []string) string {
	const hashLen = 8

	resolvedPaths := fs.RevealWildcardPaths(paths)

	digests := make([]string, 0, len(resolvedPaths))
	for _, path := range resolvedPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("cannot read config file %s for preflight cache hash: %v", path, err))
			continue
		}
		sum := sha256.Sum256(data)
		digests = append(digests, hex.EncodeToString(sum[:]))
	}
	sort.Strings(digests)

	h := sha256.New()
	for _, d := range digests {
		h.Write([]byte(d))
	}
	hash := hex.EncodeToString(h.Sum(nil))
	if len(hash) > hashLen {
		hash = hash[:hashLen]
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("computed config hash %s over %d config file(s)", hash, len(digests)))

	return hash
}
