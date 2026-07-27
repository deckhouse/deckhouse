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

// Package extra models the auxiliary image catalog of a module or a package:
//
//	<...>/<name>/extra/<extra>:<version>
//
// Modules and packages share this shape, so both use this package. Extra images
// are versioned independently of the module or package that ships them —
// neuvector/extra/scanner:3 alongside neuvector:v1.0.1, for example.
package extra

import (
	"context"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/internal/cache"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Segment is the fixed path segment of an extra image catalog.
const Segment = "extra"

// Catalog is the extra image catalog of one module or package.
type Catalog struct {
	*service.BasicService

	imageServiceName string
	images           *cache.Cache[*service.BasicService]
}

// NewCatalog builds the extra catalog under parent, appending Segment.
// catalogServiceName and imageServiceName appear in log records.
func NewCatalog(parent *service.BasicService, catalogServiceName, imageServiceName string) *Catalog {
	return &Catalog{
		BasicService:     parent.Sub(catalogServiceName, Segment),
		imageServiceName: imageServiceName,
		images:           cache.New[*service.BasicService](),
	}
}

// List returns the names of the extra images in this catalog.
func (c *Catalog) List(ctx context.Context) ([]string, error) {
	return c.ListTags(ctx)
}

// Image returns the repository of a single extra image (<...>/extra/<name>).
// Repeated calls with the same name return the same service.
func (c *Catalog) Image(name string) *service.BasicService {
	return c.images.Get(name, func() *service.BasicService {
		return c.Named(c.imageServiceName, name)
	})
}
