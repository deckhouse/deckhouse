/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/node-controller/internal/apiserver"
)

// serveAggregatedAPIChildEnv marks the re-executed test binary that runs the
// server itself: os.Exit can only be observed from another process.
const serveAggregatedAPIChildEnv = "NODE_CONTROLLER_TEST_SERVE_AGGREGATED_API"

// A replica that outlives its aggregated API server keeps port 4293 closed while
// it stays Ready and in the Endpoints the APIService points at, and the
// APIService is installed on every cluster: discovery then fails on all of them.
func TestAggregatedAPIServerDeathTakesTheProcessDown(t *testing.T) {
	if os.Getenv(serveAggregatedAPIChildEnv) == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		// No serving certificate: the config never builds, and Run gives up with
		// that error once ctx is done.
		serveAggregatedAPI(ctx, logr.Discard(), apiserver.Options{
			BindPort: apiserverPort,
			CertFile: "/nonexistent/tls.crt",
			KeyFile:  "/nonexistent/tls.key",
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestAggregatedAPIServerDeathTakesTheProcessDown", "-test.timeout=1m")
	cmd.Env = append(os.Environ(), serveAggregatedAPIChildEnv+"=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "the process survived its aggregated API server")
	require.Equal(t, 1, exitErr.ExitCode())
}

// The manager's probe answers on its own port, and the aggregated API server
// starts on another one behind a retry loop. Until readiness reads that second
// port, the pod joins the Endpoints the APIService routes to while nothing is
// answering there, and every full-discovery client in the cluster fails.
func TestReadinessWaitsForTheAggregatedAPIServer(t *testing.T) {
	serving := make(chan struct{})
	ready := aggregatedAPIReady(serving, time.Minute)

	require.Error(t, ready(nil), "the pod was Ready before its aggregated API server answered")

	close(serving)
	require.NoError(t, ready(nil))
}

// The same readiness gates the five failurePolicy:Fail webhooks and the NodeGroup
// conversion webhook, on the one replica a non-HA cluster runs. A gate with no
// end turns an aggregated API server that cannot start — a missing RBAC binding
// is enough — into a cluster that refuses every NodeGroup write, forever: the
// config retry has no cap, and nothing but SIGTERM ends the process.
func TestReadinessStopsWaitingForTheAggregatedAPIServer(t *testing.T) {
	ready := aggregatedAPIReady(make(chan struct{}), 20*time.Millisecond)

	require.Error(t, ready(nil), "the pod was Ready before its aggregated API server answered")

	require.Eventually(t, func() bool { return ready(nil) == nil }, time.Second, 5*time.Millisecond,
		"the webhooks stayed NotReady for as long as the aggregated API server did not answer")
}
