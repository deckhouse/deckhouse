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

// Package security models the security image catalog:
//
//	<root>/<edition>/security/<name>:<version>
//
// Its images are security data bundles versioned on their own schedule —
// security/trivy-db:2, for example.
package security

import (
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/internal/cache"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Segment is the fixed path segment of the security catalog.
const Segment = "security"

// Service names used in log records.
const (
	CatalogServiceName = "security"
	imageServiceName   = "security_image"
)

// Catalog is the security image catalog at <root>/<edition>/security.
type Catalog struct {
	*service.BasicService

	images *cache.Cache[*service.BasicService]
}

// NewCatalog wraps a repository that already addresses the security catalog.
// The assembler supplies the path via Sub(CatalogServiceName, Segment).
func NewCatalog(svc *service.BasicService) *Catalog {
	return &Catalog{
		BasicService: svc,
		images:       cache.New[*service.BasicService](),
	}
}

// Image returns the repository of a single security image
// (<root>/<edition>/security/<name>). Repeated calls with the same name return
// the same service.
func (c *Catalog) Image(name string) *service.BasicService {
	return c.images.Get(name, func() *service.BasicService {
		return c.Named(imageServiceName, name)
	})
}
