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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	labels "k8s.io/apimachinery/pkg/labels"
)

type MountPointInfo struct {
	Path       string  `json:"path"`
	Mountpoint *string `json:"mountpoint"`
}

func waitForRegistryStaticPodDeletion(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Checking registry static pod on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return checkRegistryStaticPodExistence(kubeClient, nodeName)
	})
}

func attemptUnmountRegistryData(kubeClient *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(
		fmt.Sprintf("Attempting to unmount registry data device on node '%s'", nodeName),
		45, 10*time.Second,
	).Run(func() error {
		return unmountRegistryData(kubeClient, nodeName)
	})
}

func unmountRegistryData(kubeClient *client.KubernetesClient, nodeName string) error {
	const mountPoint = "/mnt/system-registry-data"

	sshClient := kubeClient.NodeInterfaceAsSSHClient()
	if sshClient == nil {
		return fmt.Errorf("failed to obtain SSH client")
	}

	host := ""
	for _, availableHost := range sshClient.Settings.AvailableHosts() {
		if availableHost.Name == nodeName {
			host = availableHost.Host
		}
	}
	if host == "" {
		return fmt.Errorf("node '%s' not found in available hosts", nodeName)
	}

	settings := sshClient.Settings.Copy()
	settings.SetAvailableHosts([]session.Host{{Host: host, Name: nodeName}})

	customSSHClient := ssh.NewClient(settings, sshClient.PrivateKeys)

	exists, err := isMountPointPresent(mountPoint, customSSHClient)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	return unmountPath(mountPoint, sshClient)
}

func isMountPointPresent(mountPoint string, sshClient *ssh.Client) (bool, error) {
	nodeWrapper := ssh.NewNodeInterfaceWrapper(sshClient)
	stdout, stderr, err := nodeWrapper.Command(
		"bash", "-c",
		"lsblk -o path,type,mountpoint,fstype --tree --json | jq -r '[.blockdevices[] | select(.mountpoint != null) ]'",
	).Output()
	if err != nil {
		return false, fmt.Errorf("command error: %s %s", string(stderr), err)
	}

	var mountPoints []MountPointInfo
	if err := json.Unmarshal(stdout, &mountPoints); err != nil {
		return false, fmt.Errorf("failed to unmarshal lsblk output: %s", err)
	}

	for _, mountInfo := range mountPoints {
		if mountInfo.Mountpoint != nil && *mountInfo.Mountpoint == mountPoint {
			return true, nil
		}
	}
	return false, nil
}

func unmountPath(mountPoint string, sshClient *ssh.Client) error {
	nodeWrapper := ssh.NewNodeInterfaceWrapper(sshClient)
	cmd := nodeWrapper.Command("umount", mountPoint)
	cmd.Sudo()
	cmd.WithTimeout(10 * time.Second)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unmount path '%s': %s %s", mountPoint, string(cmd.StderrBytes()), err)
	}
	return nil
}

func checkRegistryStaticPodExistence(kubeClient *client.KubernetesClient, nodeName string) error {
	const podNamespace = "d8-system"
	podLabels := map[string]string{
		"component": "system-registry",
		"tier":      "control-plane",
	}

	staticPods, err := kubeClient.CoreV1().Pods(podNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(podLabels).String(),
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			log.InfoF("No static pods found on node '%s'. Skipping.", nodeName)
			return nil
		}
		return fmt.Errorf("failed to list pods on node '%s': %v", nodeName, err)
	}

	if len(staticPods.Items) > 0 {
		var podNames []string
		for _, pod := range staticPods.Items {
			podNames = append(podNames, pod.Name)
		}
		return fmt.Errorf(
			"found static pods '%v' on node '%s'. Please delete them manually",
			podNames, nodeName,
		)
	}
	return nil
}
