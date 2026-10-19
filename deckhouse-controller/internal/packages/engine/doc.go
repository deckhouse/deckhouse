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

// Package engine is the package engine of deckhouse-controller: it installs, updates, configures,
// disables and removes applications and, with EnableModuleV2, modules and Global; without it it
// runs applications only, beside addon-operator.
//
//	engine/            the composition root and the facade. build.go wires the services and the
//	                   core and orders their start and stop (read it first); services.go and
//	                   load.go build the services, the scheduler and the loader; facade.go is the
//	                   API the controllers call; sink.go routes notifications to the status
//	                   queues and the settings channels; admission.go serves the validation
//	                   webhooks; debug.go the dumps of the debug API; orphans.go and webhooks.go
//	                   adapt services to the core's interfaces.
//	engine/core/       the registry, the dispatcher, readiness and the boot.
//	engine/lifecycle/  the package Reconciler, one per package name: it owns Current and the
//	                   Candidate and decides the version (start with reconciler.go and loop.go,
//	                   then prepare.go, activate.go and remove.go). Its sub-packages:
//	  current/         Current, one per working revision: accept, run, disable, uninstall
//	                   (start with next.go, then loop.go).
//	  global/          Global: the global module and the barrier (EnableModuleV2 only).
//	  hooklane/        the hook lanes of Current and Global and their Router (lanes.go).
//	  gate/            gate.Decide, the package Reconciler's pure update decision.
//	  model/           the values the components hand each other: Desired, Revision, Report.
//	  configstate/     the configuration state Current and Global share.
//	  inbox/, operation/, report/, retry/
//	                   the loops' primitives: latest-value inputs, the operation worker, the
//	                   Report store, the backoff and the clock.
//	engine/schedule/   the scheduler: the convergence protocol and the graph (protocol.go,
//	                   then scheduler.go); rule/ holds the rules a node's decision folds.
//	engine/enginetest/ the scenario kit over fakes; tests only.
//
// core, lifecycle and schedule make up the core and, by convention, import no Kubernetes client:
// every cluster effect goes through an interface its consumer declares, implemented by a service
// this package builds. One cluster read happens inside a scheduler computation: in the default
// tree the dependency getter (services.go, addonDependencyGetter) reads the Module of an
// application's dependency from the manager's cached client.
//
// A package's status has two halves, joined by name in the status projection: Engine.Report, the
// lifecycle half (decisions and the results of operations), and Engine.Observed, the observed half
// (internal/packages/observed: what the release service, the health service and the hook lanes
// observe). The hook lanes, which run inside the lifecycle, write a hook failure there; the rest of
// the core does not touch it. The facade forgets a package's observations when it reports its
// removal done.
package engine
