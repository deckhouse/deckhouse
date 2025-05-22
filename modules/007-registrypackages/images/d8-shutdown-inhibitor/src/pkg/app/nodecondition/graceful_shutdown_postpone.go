/*
Copyright 2025 Flant JSC

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

package nodecondition

import (
	"fmt"
	"os/exec"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

const (
	GracefulShutdownPostponeType = "GracefulShutdownPostpone"
	ReasonOnStart                = "ShutdownInhibitorIsStarted"
	ReasonOnUnlock               = "NoRunningPodsWithLabel"
	ReasonPodsArePresent         = "PodsWithLabelAreRunningOnNode"
)

func GracefulShutdownPostpone() *gracefulShutdownPostpone {
	return &gracefulShutdownPostpone{}
}

type gracefulShutdownPostpone struct{}

func (g *gracefulShutdownPostpone) SetOnStart(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, StatusTrue, ReasonOnStart)
}

func (g *gracefulShutdownPostpone) SetPodsArePresent(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, StatusTrue, ReasonPodsArePresent)
}

func (g *gracefulShutdownPostpone) UnsetOnUnlock(nodeName string) error {
	return patchGracefulShutdownPostponeCondition(nodeName, StatusFalse, ReasonOnUnlock)
}

// patchGracefulShutdownPostponeCondition updates GracefulShutdownPostpone condition.
/*
kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"True", "reason":"PodsWithLabelAreRunningOnNode"}]}}'
--subresource=status

kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"False", "reason":"NoRunningPodsWithLabel"}]}}'
--subresource=status
*/
func patchGracefulShutdownPostponeCondition(nodeName, status, reason string) error {
	k := kubernetes.NewDefaultKubectl()
	err := k.PatchCondition("Node", nodeName, GracefulShutdownPostponeType, status, reason, "")
	return reformatExitError(err)
}

func reformatExitError(err error) error {
	if err == nil {
		return nil
	}
	ee, ok := err.(*exec.ExitError)
	if ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%v: %s", err, string(ee.Stderr))
	}
	return err
}
