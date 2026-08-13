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

import (
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// What the bootstrap leaves in the state cache.
const (
	PostBootstrapResultCacheKey      = "post-bootstrap-result"
	ManifestCreatedInClusterCacheKey = "tf-state-and-manifests-in-cluster"
	BastionHostCacheKey              = "bastion-host"
)

// cacheKeysToKeep survive the cache cleanup of a finished bootstrap: they are
// what converge and destroy read afterwards. Everything else is bootstrap's own
// bookkeeping and a next run starts a new cluster with it.
var cacheKeysToKeep = []string{
	state.MasterHostsCacheKey,
	ManifestCreatedInClusterCacheKey,
	BastionHostCacheKey,
	PostBootstrapResultCacheKey,
}

// What a joining node reads out of the running cluster. Each of these names an
// object node-controller also addresses; see the mirrors named below.
const (
	// The kubernetes Service's EndpointSlice, the cluster's own record of where
	// its apiservers answer. Named the same way node-controller names them.
	apiServerEndpointSliceNS   = "default"
	apiServerEndpointSliceName = "kubernetes"
	apiServerPortName          = "https"

	// clusterCAConfigMap carries the cluster CA every ServiceAccount is given.
	// node-controller renders day-2 configs from the same source, so a node
	// bootstrapped here and the same node reconciled later see one CA.
	// Mirrors node-controller/src/internal/controller/nodeconfig/constants.go.
	clusterCAConfigMap = "kube-root-ca.crt"
	clusterCAKey       = "ca.crt"

	// bootstrapTokenNGLabel labels a bootstrap-token secret with the NodeGroup
	// it belongs to.
	// Mirrors node-controller/src/internal/controller/nodebootstrap/constants.go.
	bootstrapTokenNGLabel = "node-manager.deckhouse.io/node-group"
)

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

	// Everything a joining node needs is published by a Deckhouse hook after the
	// NodeGroup arrives, so the first read of a young cluster finds nothing. The
	// budget is the classic path's, which waits this long for the group's cloud
	// config (entity.GetCloudConfig).
	waitJoinInputs = waitBudget{attempts: 225, interval: time.Second}
)
