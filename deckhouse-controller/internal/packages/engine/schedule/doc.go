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

// Package schedule is the scheduler: one graph of every package node (modules, applications and
// Global) and one computation that answers, for every node at once, "may it run now?".
//
// Facts in, levels out (protocol.go): a Current or Global publishes only facts about its own node
// (Publish); the scheduler computes, atomically under one mutex, every node's state and delivers it
// as one latest Event per node, signalled by Wake and read in batches by Take. A lost or repeated
// message changes nothing: the next one carries the whole state. Per computation:
//
//	decision     the node's rule chain folded by rule.Resolve. The position is the precedence:
//	             the floor, then the requirement gates, which can only forbid, then the intent
//	             rules (Module.spec.enabled, the global hooks' signal, the enabled script).
//	inputs       a hash of the decision, the node's own inputs and the Output of the nodes it
//	             subscribes to; Global's also name every enabled module's revision.
//	convergence  disabled, or the node applied exactly these inputs.
//	gate         enabled, and Global, every lower tier, every dependency and every kept AnyOf
//	             member are converged.
//
// The scheduler starts paused and computes nothing before the boot gate (Resume): an unknown input
// reads as Forbid, so an earlier computation would disable modules and delete their releases.
// Recheck recomputes for inputs no fact carries (addon-operator's state, a global hook's signal).
//
//	protocol.go   Facts, Event and the errors the admission checks return.
//	scheduler.go  the Scheduler: its inputs, the computation, the gate.
//	node.go       Node, Constraints and the rule chain of a node (addNode).
//	event.go      delivery: the latest-value slots, Wake and Take.
//	topo.go       the dependency order and cycle detection.
//	dump.go       the debug dump.
//	rule/         the rule families a chain is built from.
//
// Rules and getters run inside the computation under the mutex: they must not call back into the
// scheduler, and a slow one holds every package. The package never imports the engine.
package schedule
