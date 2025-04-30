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

package kubernetes

import (
	"fmt"
	"os/exec"
)

const KubectlPath = "/opt/deckhouse/bin/kubectl"
const KubeConfigPath = "/etc/kubernetes/kubelet.conf"

type Kubectl struct {
	kubectlPath    string
	kubeConfigPath string
}

func NewDefaultKubectl() *Kubectl {
	return NewKubectlWithConf(KubectlPath, KubeConfigPath)
}

func NewKubectlWithConf(kubectlPath, kubeConfigPath string) *Kubectl {
	return &Kubectl{
		kubeConfigPath: kubeConfigPath,
		kubectlPath:    kubectlPath,
	}
}

func (k *Kubectl) Cordon(nodeName string) ([]byte, error) {
	cmd := k.cmd("cordon", nodeName)
	return cmd.CombinedOutput()
}

func (k *Kubectl) ListPods(nodeName string) (*PodList, error) {
	out, err := k.listPods(nodeName)
	if err != nil {
		return nil, err
	}

	podList, err := podsListFromJSON(out)
	if err != nil {
		return nil, err
	}

	return podList, nil
}

func (k *Kubectl) listPods(nodeName string) ([]byte, error) {
	nodeNameFieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	cmd := k.cmd("get", "po", "-A", "-o", "json", "--field-selector", nodeNameFieldSelector)
	return cmd.Output()
}

func (k *Kubectl) PatchCondition(kind, name, condType, status, reason, message string) error {
	patch := fmt.Sprintf(`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message)
	return k.patchStatusStrategic(kind, name, patch)
}

func (k *Kubectl) patchStatusStrategic(kind, name, patch string) error {
	cmd := k.cmd("patch", kind, name, "--subresource=status", "--type", "strategic", "-p", patch)
	_, err := cmd.Output()
	return err
}

func (k *Kubectl) cmd(args ...string) *exec.Cmd {
	kArgs := append([]string{}, "--kubeconfig", KubeConfigPath)
	kArgs = append(kArgs, args...)
	return exec.Command(k.kubectlPath, kArgs...)
}
