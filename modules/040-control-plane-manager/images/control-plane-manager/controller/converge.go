/*
Copyright 2023 Flant JSC

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
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func installExtraFiles() error {
	dstDir := filepath.Join(deckhousePath, "extra-files")
	log.Infof("phase: install extra files to %s", dstDir)

	if err := removeDirectory(dstDir); err != nil {
		return err
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(configPath)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "extra-file-") {
			continue
		}

		if err := installFileIfChanged(filepath.Join(configPath, entry.Name()), filepath.Join(dstDir, strings.TrimPrefix(entry.Name(), "extra-file-")), 0644); err != nil {
			return err
		}
	}
	return nil
}

func convergeComponents() error {
	log.Infof("phase: converge kubernetes components")
	for _, v := range []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler", "etcd"} {
		if err := convergeComponent(v); err != nil {
			return err
		}
	}
	return nil
}

func convergeComponent(componentName string) error {
	log.Infof("converge component %s", componentName)

	//remove checksum patch, if it was left from previous run
	_ = os.Remove(filepath.Join(deckhousePath, "kubeadm", "patches", componentName+"999checksum.yaml"))

	if err := prepareConverge(componentName, true); err != nil {
		return err
	}

	checksum, err := calculateChecksum(componentName)
	if err != nil {
		return err
	}

	recreateConfig := false
	if _, err := os.Stat(filepath.Join(manifestsPath, componentName+".yaml")); err == nil {
		equal, err := manifestChecksumIsEqual(componentName, checksum)
		if err != nil {
			return err
		}
		if !equal {
			recreateConfig = true
		}
	} else {
		recreateConfig = true
	}

	if recreateConfig {
		log.Infof("generate new manifest for %s", componentName)
		if err := backupFile(filepath.Join(manifestsPath, componentName+".yaml")); err != nil {
			log.Warnf("Backup failed, %s", err)
		}

		if err := generateChecksumPatch(componentName, checksum); err != nil {
			return err
		}

		_, err := os.Stat("/var/lib/etcd/member")
		if componentName == "etcd" && err != nil {
			if err := etcdJoinConverge(); err != nil {
				return err
			}
		} else {
			if err := prepareConverge(componentName, false); err != nil {
				return err
			}
		}

		_ = os.Remove(filepath.Join(deckhousePath, "kubeadm", "patches", componentName+"999checksum.yaml"))

	} else {
		log.Infof("skip manifest generation for component %s because checksum in manifest is up to date", componentName)
	}

	return waitPodIsReady(componentName, checksum)
}

func prepareConverge(componentName string, isTemp bool) error {
	args := []string{"init", "phase"}
	if componentName == "etcd" {
		// kubeadm init phase etcd local --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
		args = append(args, "etcd", "local", "--config", deckhousePath+"/kubeadm/config.yaml")
	} else {
		// kubeadm init phase control-plane apiserver --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
		args = append(args, "control-plane", strings.TrimPrefix(componentName, "kube-"), "--config", deckhousePath+"/kubeadm/config.yaml")
	}
	if isTemp {
		args = append(args, "--rootfs", config.TmpPath)
	}
	c := exec.Command(kubeadmPath, args...)
	out, err := c.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Infof("%s", s)
	}
	return err
}

func calculateChecksum(componentName string) (string, error) {
	h := sha256.New()
	manifest, err := os.ReadFile(filepath.Join(config.TmpPath, manifestsPath, componentName+".yaml"))
	if err != nil {
		return "", err
	}

	if _, err := h.Write(manifest); err != nil {
		return "", err
	}

	re := regexp.MustCompile(`=(/etc/kubernetes/.+)`)
	res := re.FindAllSubmatch(manifest, -1)

	filesMap := make(map[string]struct{}, len(res))

	for _, v := range res {
		filesMap[string(v[1])] = struct{}{}
	}

	filesSlice := make([]string, len(filesMap))
	i := 0
	for k := range filesMap {
		filesSlice[i] = k
		i++
	}

	sort.Strings(filesSlice)

	for _, file := range filesSlice {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		if _, err := h.Write(content); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func manifestChecksumIsEqual(componentName, checksum string) (bool, error) {
	content, err := os.ReadFile(filepath.Join(manifestsPath, componentName+".yaml"))
	if err != nil {
		return false, err
	}
	return strings.Index(string(content), checksum) != -1, nil
}

func generateChecksumPatch(componentName string, checksum string) error {
	const patch = `apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/checksum: "%s"`
	log.Infof("write checksum patch for component %s", componentName)
	patchFile := filepath.Join(deckhousePath, "kubeadm", "patches", componentName+"999checksum.yaml")
	content := fmt.Sprintf(patch, componentName, checksum)
	return os.WriteFile(patchFile, []byte(content), 0644)
}

func etcdJoinConverge() error {
	// kubeadm -v=5 join phase control-plane-join etcd --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
	args := []string{"-v=5", "join", "phase", "control-plane-join", "etcd", "--config", deckhousePath + "/kubeadm/config.yaml"}
	c := exec.Command(kubeadmPath, args...)
	out, err := c.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Infof("%s", s)
	}
	return err
}

func waitPodIsReady(componentName string, checksum string) error {
	tries := 0
	log.Infof("waiting for the %s pod component to be ready with the new manifest in apiserver", componentName)
	for {
		tries++
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		podName := fmt.Sprintf("%s-%s", componentName, config.NodeName)
		pod, err := config.K8sClient.CoreV1().Pods("kube-system").Get(ctx, podName, metav1.GetOptions{})
		cancel()
		if err != nil {
			log.Warn(err)
		}

		if tries > maxRetries {
			return fmt.Errorf("timeout waiting for pod %s to become ready with expected checksum %s", podName, checksum)
		}

		attemptReReadManifest := maxRetries - 60

		if tries == attemptReReadManifest {
			err := triggerKubeletRereadManifest(componentName)
			// https://github.com/kubernetes/kubernetes/issues/109596
			if err != nil {
				return fmt.Errorf("fail to trigger re-read manifest for %s: %s", componentName, err)
			}
		}

		if podChecksum := pod.Annotations["control-plane-manager.deckhouse.io/checksum"]; podChecksum != checksum {
			log.Warnf("kubernetes pod %s checksum %s does not match expected checksum %s", podName, podChecksum, checksum)
			time.Sleep(1 * time.Second)
			continue
		}
		var podIsReady bool
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				podIsReady = true
				break
			}
		}
		if !podIsReady {
			log.Warnf("kubernetes pod %s has matching checksum %s but is not ready", podName, checksum)
			time.Sleep(1 * time.Second)
			continue
		}

		log.Infof("kubernetes pod %s has matching checksum %s and is ready", podName, checksum)
		return nil
	}
}

func triggerKubeletRereadManifest(componentName string) error {
	log.Warnf("trying to trigger kubelet to re-read manifest")

	srcPath := filepath.Join(manifestsPath, componentName+".yaml")
	dstPath := filepath.Join(manifestsPath, "."+componentName+".yaml")

	if err := copy.Copy(srcPath, dstPath); err != nil {
		return err
	}

	if err := os.Remove(srcPath); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	if err := copy.Copy(dstPath, srcPath); err != nil {
		return err
	}

	return nil
}
