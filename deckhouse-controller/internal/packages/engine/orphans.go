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

package engine

import (
	"context"
	"log/slog"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/orphans"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// orphanOwner is a package object that declares what it leaves beside its release. Only
// applications implement it, so the purge asserts it on the carrier.
type orphanOwner interface {
	GetName() string
	GetNamespace() string
	GetInstance() string
	GetOrphanResources() ([]orphans.Resource, error)
}

// Applications declare their leftovers; modules do not.
var _ orphanOwner = (*apps.Application)(nil)

// orphanPurge is current.Deps.Orphans over orphans.Service. It lives here so the core imports
// neither the service nor its resource type.
type orphanPurge struct {
	service *orphans.Service
	logger  *log.Logger
}

// Purge deletes what pkg declares it leaves beside its release. A declaration that cannot be read
// is logged and skipped, not failed: it reads the same on every retry, and the removal would wait
// forever.
func (p orphanPurge) Purge(ctx context.Context, pkg model.Package) error {
	owner, ok := pkg.(orphanOwner)
	if !ok {
		return nil
	}

	resources, err := owner.GetOrphanResources()
	if err != nil {
		p.logger.Error("skip purge, orphan resources are malformed", slog.String("package", owner.GetName()), log.Err(err))

		return nil
	}

	if len(resources) == 0 {
		return nil
	}

	return p.service.Delete(ctx, owner.GetNamespace(), owner.GetInstance(), resources)
}
