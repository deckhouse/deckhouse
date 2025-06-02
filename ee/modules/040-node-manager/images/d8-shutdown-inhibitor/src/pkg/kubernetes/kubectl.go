/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
