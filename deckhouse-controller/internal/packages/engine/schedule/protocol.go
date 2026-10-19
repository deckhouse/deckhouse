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

package schedule

import "fmt"

// The convergence protocol. A Current (or Global) publishes only its own facts about its node; the
// scheduler alone computes, atomically under one mutex, every node's decision, inputs, convergence
// and gate, and publishes the result as a level per node (Event). Only levels cross the boundary,
// so a lost or repeated message changes nothing. This package never imports the engine.

// GlobalNode is the name of the global node. Its inputs carry the identity of every enabled module,
// and every other node waits for it to be converged (the global barrier, see gateOf).
const GlobalNode = "global"

// ReasonDependencyCycle is the reason of a rejection by AddNode or CheckConstraints: the node
// would close a dependency cycle.
const ReasonDependencyCycle = "DependencyCycle"

// ReasonDependentRequirementsUnmet is the reason of a rejection by CheckDependents: an enabled
// package that depends on the node does not accept its new version.
const ReasonDependentRequirementsUnmet = "DependentRequirementsUnmet"

// Facts is what a Current publishes about its own node; facts of another revision are ignored,
// and a replaced node drops its facts.
type Facts struct {
	Version string // Version must equal the node's Revision.
	// OwnInputs is the hash of the node's own inputs. The scheduler mixes it into the inputs and
	// echoes it in the event, so the Current can tell which of its inputs an event reflects.
	OwnInputs string
	// Applied is the Inputs of the last successful run or disable; empty when unknown.
	Applied string
	// Output is the checksum of what the node publishes for its subscribers, whose inputs hash it:
	// Global's is its values after the publish step, a module Current's its image digests.
	Output string
}

// Event is the computed state of one node, produced for every node on every computation; the
// engine takes the latest per node in batches (Wake, Take).
type Event struct {
	Name string
	// Version is the node's revision; a receiver ignores an event of another revision.
	Version string
	Enabled bool
	// Reason and Message explain a negative decision; empty when enabled.
	Reason  string
	Message string
	// OwnInputs echoes the Facts.OwnInputs the event was computed from.
	OwnInputs string
	// Inputs is the hash of everything a run of the node depends on (inputsOf).
	Inputs string
	Gate   bool // Gate: enabled, and everything the node waits for is converged (gateOf).
	// EnabledModules is set only for the global node: every enabled module, sorted by ID.
	// Applications are not listed: their CRDs are not Global's.
	EnabledModules []EnabledModule
}

// EnabledModule is one enabled module in Global's event. It is built from the node in the same
// computation as the list, so the list and the attachments always describe the same revisions.
type EnabledModule struct {
	// ID is name@revision; Global's inputs hash it.
	ID string
	// Attachment is the node's Node.Attachment, passed through untouched.
	Attachment any
}

// ConstraintError is why a node would not be admitted (CheckConstraints, AddNode).
type ConstraintError struct {
	Reason  string // Reason is a Kubernetes condition reason.
	Message string

	// cause is the typed error behind the rejection (*CycleError), for errors.As.
	cause error
}

func (e *ConstraintError) Error() string {
	return fmt.Sprintf("%s: %s", e.Reason, e.Message)
}

// Unwrap returns the typed cause (*CycleError for a cycle), or nil.
func (e *ConstraintError) Unwrap() error {
	return e.cause
}
