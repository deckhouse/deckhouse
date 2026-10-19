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

// Package lifecycle is the package Reconciler, one per package name: it decides which revision of
// the package runs and owns the two things that run it, Current (the running revision,
// lifecycle/current) and the Candidate (the preparation of the desired one). A new revision is
// downloaded, loaded and checked while Current keeps working; Current stops only after every check
// passed.
//
// The loop wakes on an input from the core (Apply, Remove), a scheduler event, the end of a
// preparation attempt or its one timer; it pushes the latest configuration and event to Current,
// which interprets them itself, and decides with gate.Decide, a pure function over one snapshot:
//
//	UpToDate  the desired version is Current's; a Candidate is dropped.
//	Prepare   attach and load the desired revision; a failed attempt is retried with backoff.
//	Blocked   a requirement, the settings or the update window stop it. The prepared Candidate is
//	          kept and the timer wakes the gate again (the window's start, a retry).
//	Activate  every gate passed: the Candidate becomes Current.
//
// A file is a lifecycle stage:
//
//	reconciler.go  the type, its inputs and Start, which starts a restored package from its
//	               checkpoint.
//	loop.go        the loop and reconcile: one decision per wake, the one timer; the settings memo.
//	prepare.go     the Candidate: attach, load, the retry backoff.
//	activate.go    the switch: AddNode first, stop the old Current, start the new one, detach the
//	               old image. The point of no return; there is no rollback. Restore, the same
//	               preparation once at Boot.
//	remove.go      removal: wait for an event of Current's revision, RemoveNode, uninstall,
//	               detach, report Removed.
//
// Invariants: at most one Current and one Candidate; the package has a scheduler node exactly while
// it has a Current; settings reach Current at once, also while a Candidate is blocked; instances
// are name@version, so a Candidate never touches Current's image, and a detach keeps the stored
// image; Shutdown stops everything and changes nothing, neither the cluster nor the Report.
package lifecycle
