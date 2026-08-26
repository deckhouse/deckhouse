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

package bootstrap

import "time"

// waitBudget is how long dhctl keeps asking: attempts spaced by interval.
type waitBudget struct {
	attempts int
	interval time.Duration
}

// What the immutable path waits for. The numbers differ by an order of
// magnitude, so each says what it is waiting on.
var (
	// 30 minutes, because nothing has happened on the VM yet: it still has to
	// install its OS, reboot, pull three system extensions, start kubelet,
	// generate the PKI and pull four control-plane images before it can answer.
	waitAPIServerUp = waitBudget{attempts: 360, interval: 5 * time.Second}

	// The client's own wait for /version — a restarting static pod or a rebuilt
	// forward, not the install, which is over by then. Five minutes.
	waitAPIServerReady = waitBudget{attempts: 60, interval: 5 * time.Second}

	// Everything after the apiserver answers. Registering the Node is the node's
	// next step, so a couple of minutes is generous.
	waitNodeRegistered = waitBudget{attempts: 120, interval: time.Second}

	// The machine may be powering on when the bootstrap starts, and olcedar-init
	// opens the port about thirty seconds into the boot. Ten minutes.
	waitMaintenancePort = waitBudget{attempts: 120, interval: 5 * time.Second}

	// The preflight asks a different question than the wait above: not "has this
	// machine finished booting" but "did the operator name machines that exist".
	// Three tries, and the whole thing is over in about ten seconds: an address
	// nobody answers for is a typo, and a typo must not be waited out.
	checkMachinesWaiting = waitBudget{attempts: 3, interval: 2 * time.Second}
)

// checkMachineTimeout bounds one try of the preflight above. Without it a try
// runs to the HTTP client's own 30s: an address that swallows packets — which is
// what a typo in a private network looks like — then costs minutes, not seconds.
const checkMachineTimeout = 3 * time.Second
