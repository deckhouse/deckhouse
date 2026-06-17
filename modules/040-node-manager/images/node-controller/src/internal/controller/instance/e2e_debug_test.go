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

package instance_controller

import (
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/deckhouse/node-controller/internal/testenv"
)

// A skipped spec documenting how to inspect the envtest cluster interactively. The envtest
// apiserver only runs during the suite, so a spec must hold it open. To use it, change XIt to
// FIt (focus) and run:
//
//	ENVTEST_DEBUG=1 make test-envtest
//
// then, while it is paused, in another terminal:
//
//	KUBECONFIG=/tmp/envtest.kubeconfig kubectl get instances,nodes -A -o wide
//	KUBECONFIG=/tmp/envtest.kubeconfig kubectl get machines.cluster.x-k8s.io -A
var _ = Describe("envtest debugging", func() {
	XIt("pauses so the cluster can be inspected with real kubectl", func() {
		createStaticNode(uniqueName("debug"))
		dumpAll("debug snapshot")
		testenv.PauseForKubectl(GinkgoWriter, testEnv, cfg, 2*time.Minute)
	})
})
