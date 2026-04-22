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

package bundle

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store/oci"
)

// extractOCIStore walks root, finds every OCI layout marker file,
// validates and opens each layout as a Store, and returns all stores
// keyed by their layout path (relative to root). Layouts without tags
// are skipped. An optional transform function rewrites each layout path key.
func extractOCIStore(ctx context.Context, root fs.FS, transformLayoutPath func(string) string) (repoStores, error) {
	ret := make(repoStores)

	err := fs.WalkDir(root, ".", func(relPath string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if ent.IsDir() || ent.Name() != ociv1.ImageLayoutFile {
			return nil
		}

		layoutPath := path.Dir(relPath)
		subFS, err := fs.Sub(root, layoutPath)
		if err != nil {
			return fmt.Errorf("make subfs from %q: %w", layoutPath, err)
		}

		if err := oci.ValidateLayout(ctx, subFS); err != nil {
			return fmt.Errorf("validate oci layout in subfs %q: %w", layoutPath, err)
		}

		st, err := oci.NewLayoutStore(subFS)
		if err != nil {
			return fmt.Errorf("new oci store form subfs %q: %w", layoutPath, err)
		}

		if !st.HasTags() {
			return nil
		}

		if layoutPath == "." {
			layoutPath = ""
		}

		if transformLayoutPath != nil {
			layoutPath = transformLayoutPath(layoutPath)
		}
		ret[layoutPath] = st
		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

// extractLegacyStore wraps extractOCIStore and rewrites store keys
// from older archive naming conventions to canonical registry repository paths.
func extractLegacyStore(ctx context.Context, root fs.FS, archName string) (repoStores, error) {
	return extractOCIStore(ctx, root, func(layoutPath string) string {
		return legacyPathTransform(archName, layoutPath)
	})
}

// legacyPathTransform rewrites a layout path from older archive naming conventions
// to a canonical registry repository path.
func legacyPathTransform(archFileName, layoutPath string) string {
	switch {
	case strings.HasPrefix(archFileName, SecurityFilePrefix) &&
		!strings.HasPrefix(layoutPath, SecurityRootPath):
		return path.Join(SecurityRootPath, layoutPath)

	case strings.HasPrefix(archFileName, ModuleFilePrefix) &&
		!strings.HasPrefix(layoutPath, ModulesRootPath):
		moduleName := strings.TrimPrefix(archFileName, ModuleFilePrefix)
		moduleName, _, _ = strings.Cut(moduleName, ".")
		return path.Join(ModulesRootPath, moduleName, layoutPath)

	default:
		return layoutPath
	}
}
