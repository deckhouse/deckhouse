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

// Package v1 composes the versioned endpoint tree out of the domain subtrees.
package v1

import (
	"github.com/go-chi/chi/v5"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1/packages"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1/queues"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1/requirements"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/debug/handlers/v1/scheduler"
)

// Deps carries the state providers the endpoints read from.
type Deps struct {
	Packages     packages.Provider
	Queues       queues.Provider
	Scheduler    scheduler.Provider
	Requirements requirements.Source
}

// NewPublicRouter composes the subtrees whose responses carry no package values:
// queues, scheduler and requirements.
func NewPublicRouter(deps Deps) chi.Router {
	router := chi.NewRouter()

	router.Mount("/queues", queues.New(deps.Queues).Routes())
	router.Mount("/scheduler", scheduler.New(deps.Scheduler).Routes())
	router.Mount("/requirements", requirements.New(deps.Requirements).Routes())

	return router
}

// NewPrivateRouter composes the public subtrees plus the packages subtree, whose
// responses carry registry credentials, rendered Secrets and hook snapshots.
func NewPrivateRouter(deps Deps) chi.Router {
	router := NewPublicRouter(deps)

	router.Mount("/packages", packages.New(deps.Packages).Routes())

	return router
}
