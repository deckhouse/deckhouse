/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	KubectlPath           = "/opt/deckhouse/bin/kubectl"
	KubeConfigPath        = "/etc/kubernetes/kubelet.conf"
	AdminKubeConfigPath   = "/etc/kubernetes/admin.conf"
	CordonAnnotationKey   = "node.deckhouse.io/cordoned-by"
	CordonAnnotationValue = "shutdown-inhibitor"
	NodeGroupLabelKey     = "node.deckhouse.io/group"
)

var ErrNodeIsNotNgManaged = errors.New("node is not managed by node group")

type Kubectl struct {
	kubectlPath    string
	kubeConfigPath string
}

func NewDefaultKubectl() *Kubectl {
	return NewKubectlWithConf(KubectlPath, KubeConfigPath)
}

func NewAdmintKubectl() *Kubectl {
	return NewKubectlWithConf(KubectlPath, AdminKubeConfigPath)
}

func NewKubectlWithConf(kubectlPath, kubeConfigPath string) *Kubectl {
	return &Kubectl{
		kubeConfigPath: kubeConfigPath,
		kubectlPath:    kubectlPath,
	}
}

func (k *Kubectl) Cordon(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("cordon", nodeName)
	defer cancel()
	return cmd.CombinedOutput()
}

func (k *Kubectl) Uncordon(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("uncordon", nodeName)
	defer cancel()
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

func (k *Kubectl) GetAnnotationCordonedBy(nodeName string) (string, error) {
	out, err := k.getAnnotationCordonedBy(nodeName)
	if err != nil {
		return "", err
	}

	annotationStr := fmt.Sprintf("%s", bytes.Trim(out, `"'`))

	return annotationStr, nil
}

func (k *Kubectl) getAnnotationCordonedBy(nodeName string) ([]byte, error) {
	jsonPath := fmt.Sprintf("jsonpath='{.metadata.annotations.%s}'", strings.ReplaceAll(CordonAnnotationKey, ".", `\.`))
	cmd, cancel := k.cmd("get", "node", nodeName, "-o", jsonPath)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) listPods(nodeName string) ([]byte, error) {
	nodeNameFieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	cmd, cancel := k.cmd("get", "po", "-A", "-o", "json", "--field-selector", nodeNameFieldSelector)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) PatchCondition(kind, name, condType, status, reason, message string) error {
	patch := fmt.Sprintf(`{"status":{"conditions":[{"type":"%s", "status":"%s", "reason":"%s", "message":"%s"}]}}`,
		condType, status, reason, message)
	return k.patchStatusStrategic(kind, name, patch)
}

func (k *Kubectl) patchStatusStrategic(kind, name, patch string) error {
	cmd, cancel := k.cmd("patch", kind, name, "--subresource=status", "--type", "strategic", "-p", patch)
	defer cancel()
	_, err := cmd.Output()
	return err
}

func (k *Kubectl) SetCordonAnnotation(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s=%s", CordonAnnotationKey, CordonAnnotationValue))
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) RemoveCordonAnnotation(nodeName string) ([]byte, error) {
	cmd, cancel := k.cmd("annotate", "node", nodeName, fmt.Sprintf("%s-", CordonAnnotationKey))
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) GetNodeGroup(nodeName string) (string, error) {
	out, err := k.getNodeGroup(nodeName)
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(string(out))
	ngName := strings.Trim(raw, `"'`)
	ngName = strings.TrimSpace(ngName)
	fmt.Printf("kubectl: node group value for node %q: %q\n", nodeName, ngName)
	if ngName == "" || strings.EqualFold(ngName, "<no value>") {
		fmt.Printf("kubectl: node %q is considered unmanaged (empty label)\n", nodeName)
		return "", ErrNodeIsNotNgManaged
	}
	fmt.Printf("kubectl: node %q resolved node group %q\n", nodeName, ngName)

	return ngName, nil
}

func (k *Kubectl) getNodeGroup(nodeName string) ([]byte, error) {
	escapedKey := strings.NewReplacer(".", `\.`, "/", `\/`).Replace(NodeGroupLabelKey)
	jsonPath := fmt.Sprintf("jsonpath='{.metadata.labels.%s}'", escapedKey)
	cmd, cancel := k.cmd("get", "node", nodeName, "-o", jsonPath)
	defer cancel()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get node %s: %w (output: %s)", nodeName, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (k *Kubectl) GetNodesCountInNg(nodeGroupName string) (int32, error) {
	ngJSON, err := k.getNodeGroupObject(nodeGroupName)
	if err != nil {
		return 0, err
	}

	var ng NodeGroup
	if err = json.Unmarshal(ngJSON, &ng); err != nil {
		return 0, fmt.Errorf("unmarshal nodegroup %s: %w", nodeGroupName, err)
	}

	fmt.Printf("kubectl: nodegroup %q reported nodes=%d\n", nodeGroupName, ng.Status.Nodes)
	return ng.Status.Nodes, nil
}

func (k *Kubectl) getNodeGroupObject(nodeGroupName string) ([]byte, error) {
	cmd, cancel := k.cmd("get", "nodegroup", nodeGroupName, "-o", "json")
	defer cancel()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl get nodegroup %s: %w (output: %s)", nodeGroupName, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (k *Kubectl) CountNodes() (int, error) {
	cmd, cancel := k.cmd("get", "nodes", "-o", "json")
	defer cancel()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("kubectl get nodes: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	var list NodeList
	if err := json.Unmarshal(out, &list); err != nil {
		return 0, fmt.Errorf("unmarshal nodes list: %w", err)
	}

	count := len(list.Items)
	fmt.Printf("kubectl: total nodes count = %d\n", count)
	return count, nil
}

func (k *Kubectl) GetCondition(nodeName, reason string) (*Condition, error) {
	out, err := k.getCondition(nodeName, reason)
	if err != nil {
		return nil, err
	}

	return ConditionFromJSON(out)
}

func (k *Kubectl) getCondition(nodeName, reason string) ([]byte, error) {
	jsonPath := fmt.Sprintf(`jsonpath={.status.conditions[?(@.reason=="%s")]}`, reason)
	cmd, cancel := k.cmd("get", "node", nodeName, "-o", jsonPath)
	defer cancel()
	return cmd.Output()
}

func (k *Kubectl) cmd(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	kubeConfig := k.kubeConfigPath
	if kubeConfig == "" {
		kubeConfig = KubeConfigPath
	}
	kArgs := append([]string{}, "--kubeconfig", kubeConfig)
	kArgs = append(kArgs, args...)
	return exec.CommandContext(ctx, k.kubectlPath, kArgs...), cancel
}
