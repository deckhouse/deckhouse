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

package preflightnew

import "time"

// DefaultPreflightCheckTimeout bounds one attempt of a check that asks a single question and
// waits for a single answer: a config comparison, one HTTP round trip, one TCP handshake. The
// constant existed before and was referenced nowhere, so a registry that accepted the connection
// and never answered hung the bootstrap with no output at all.
const DefaultPreflightCheckTimeout = 30 * time.Second

// NodeCheckTimeout bounds one attempt of a check that uploads a script to a node and runs it.
// The budget covers the upload as well as the run, over a link dhctl does not control.
const NodeCheckTimeout = 2 * time.Minute

// LongCheckTimeout bounds one attempt of a check that walks a list of machines, spending the
// per-machine budget of the two above on each of them in turn.
const LongCheckTimeout = 5 * time.Minute
