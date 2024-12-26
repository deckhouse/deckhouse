// Copyright 2024 Flant JSC
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

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/hashicorp/go-multierror"
	labels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

const (
	registryDataDeviceMountLockAnnotation = "embedded-registry.deckhouse.io/lock-data-device-mount"
	registryDataDeviceMountPoint          = "/mnt/system-registry-data"
	registryDataDeviceNodeLabel           = "node.deckhouse.io/registry-data-device-ready"
	registryTerraformEnableFlagVar        = "systemRegistryEnable"
	registryPodsNamespace                 = "d8-system"
	registryModuleName                    = "system-registry"
)

type MountPointInfo struct {
	Path       string  `json:"path"`
	Mountpoint *string `json:"mountpoint"`
}

func isRegistryMustBeEnabled(terraformVars []byte) (bool, error) {
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal(terraformVars, &objmap); err != nil {
		return false, nil
	}

	value, found := objmap[registryTerraformEnableFlagVar]
	if !found {
		return false, nil
	}

	var boolValue bool
	if err := json.Unmarshal(*value, &boolValue); err != nil {
		return false, err
	}
	return boolValue, nil
}

func waitForRegistryPodsDeletion(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Checking for registry pods on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		var result *multierror.Error
		if err := checkRegistryPodsExistence(
			kubeClient, nodeName, registryPodsNamespace, "registry static",
			map[string]string{
				"component": "system-registry",
				"tier":      "control-plane",
			},
		); err != nil {
			result = multierror.Append(result, err)
		}

		if err := checkRegistryPodsExistence(
			kubeClient, nodeName, registryPodsNamespace, "registry static pod manager",
			map[string]string{
				"app": "system-registry-staticpod-manager",
			},
		); err != nil {
			result = multierror.Append(result, err)
		}

		if result.ErrorOrNil() != nil {
			result = multierror.Append(
				result,
				fmt.Errorf(
					"pods of module '%s' have been detected. Before disconnecting the disks, you need to disable module '%s'",
					registryModuleName,
					registryModuleName,
				),
			)
		}

		return result.ErrorOrNil()
	})
}

func attemptUnmountRegistryData(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unmount registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		const mountPoint = registryDataDeviceMountPoint
		sshClient, err := createNodeSshClient(kubeClient, nodeName)
		if err != nil {
			return fmt.Errorf("failed to create SSH client: %s", err)
		}

		err = unsetLabels(kubeClient, nodeName, []string{registryDataDeviceNodeLabel})
		if err != nil {
			return err
		}

		exists, err := isMountPointPresent(mountPoint, sshClient)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		return umountPath(mountPoint, sshClient)
	})
}

func tryLockRegistryDataDeviceMount(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to lock mount actions for registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return setAnnotations(kubeClient, nodeName, map[string]string{registryDataDeviceMountLockAnnotation: ""})
	})
}

func tryUnlockRegistryDataDeviceMount(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unlock registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return unsetAnnotations(kubeClient, nodeName, []string{registryDataDeviceMountLockAnnotation})
	})
}

func isMountPointPresent(mountPoint string, sshClient *ssh.Client) (bool, error) {
	cmd := sshClient.Command(
		"bash", "-c", "lsblk -o path,type,mountpoint,fstype --tree --json",
	)
	cmd.Sudo()
	stdout, _, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get lsblk output: %v", err)
	}

	var result map[string][]MountPointInfo
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return false, fmt.Errorf("failed to unmarshal lsblk output: %v", err)
	}

	if blockdevices, ok := result["blockdevices"]; ok {
		for _, mountInfo := range blockdevices {
			if mountInfo.Mountpoint != nil && *mountInfo.Mountpoint == mountPoint {
				return true, nil
			}
		}
	} else {
		return false, fmt.Errorf("cannot get blockdevices field from lsblk output")
	}

	return false, nil
}

func umountPath(mountPoint string, sshClient *ssh.Client) error {
	cmd := sshClient.Command("umount", mountPoint)
	cmd.Sudo()
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to umount path '%s': %s %s", mountPoint, string(cmd.StderrBytes()), err)
	}
	return nil
}

func checkRegistryPodsExistence(kubeClient *client.KubernetesClient, nodeName, podNamespace, podName string, podLabels map[string]string) error {
	staticPods, err := kubeClient.CoreV1().Pods(podNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labels.Set(podLabels).String(),
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			log.InfoF("No '%s' pod found on node '%s'. Skipping.", podName, nodeName)
			return nil
		}
		return fmt.Errorf("failed to list '%s' pod on node '%s': %v", podName, nodeName, err)
	}
	if len(staticPods.Items) > 0 {
		var podNames []string
		for _, pod := range staticPods.Items {
			podNames = append(podNames, pod.Name)
		}
		return fmt.Errorf(
			"found '%s' pod '%v' on node '%s'",
			podName,
			podNames,
			nodeName,
		)
	}
	return nil
}

func setAnnotations(kubeClient *client.KubernetesClient, nodeName string, annotations map[string]string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()

	patchData := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	}

	for expectedValue, annotation := range annotations {
		// Check if the annotation exists
		if currentValue, ok := nodeAnnotations[annotation]; ok && expectedValue == currentValue {
			continue
		}
		patchData.ObjectMeta.Annotations[annotation] = expectedValue
	}

	if len(patchData.ObjectMeta.Annotations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(context.Background(), nodeName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to set annotations for the node: %w", err)
	}
	return nil
}

func unsetAnnotations(kubeClient *client.KubernetesClient, nodeName string, annotations []string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeAnnotations := nodeObj.GetAnnotations()

	patchOperations := make([]map[string]interface{}, 0, len(annotations))

	for _, annotation := range annotations {
		// Check if the annotation exists
		if _, ok := nodeAnnotations[annotation]; !ok {
			continue
		}

		patchOperations = append(patchOperations, map[string]interface{}{
			"op": "remove",
			// JSON patch requires slashes to be escaped with ~1
			"path": fmt.Sprintf("/metadata/annotations/%s", strings.ReplaceAll(annotation, "/", "~1")),
		})
	}

	if len(patchOperations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchOperations)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to unset annotations for the node: %w", err)
	}
	return nil
}

func unsetLabels(kubeClient *client.KubernetesClient, nodeName string, labels []string) error {
	nodeObj, err := kubeClient.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the node %s: %w", nodeName, err)
	}

	nodeLabels := nodeObj.GetLabels()

	patchOperations := make([]map[string]interface{}, 0, len(labels))

	for _, label := range labels {
		// Check if the label exists
		if _, ok := nodeLabels[label]; !ok {
			continue
		}

		patchOperations = append(patchOperations, map[string]interface{}{
			"op": "remove",
			// JSON patch requires slashes to be escaped with ~1
			"path": fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(label, "/", "~1")),
		})
	}

	if len(patchOperations) == 0 {
		return nil
	}

	patch, err := json.Marshal(patchOperations)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = kubeClient.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to unset labels for the node: %w", err)
	}
	time.Sleep(5 * time.Second)
	return nil
}

func createNodeSshClient(kubeClient *client.KubernetesClient, nodeName string) (*ssh.Client, error) {
	sshClient := kubeClient.NodeInterfaceAsSSHClient()
	if sshClient == nil {
		return nil, fmt.Errorf("failed to obtain SSH client")
	}
	host := ""
	for _, availableHost := range sshClient.Settings.AvailableHosts() {
		if availableHost.Name == nodeName {
			host = availableHost.Host
		}
	}
	if host == "" {
		return nil, fmt.Errorf("node '%s' not found in available hosts", nodeName)
	}
	settings := sshClient.Settings.Copy()
	settings.SetAvailableHosts([]session.Host{{Host: host, Name: nodeName}})
	return ssh.NewClient(settings, sshClient.PrivateKeys), nil
}
