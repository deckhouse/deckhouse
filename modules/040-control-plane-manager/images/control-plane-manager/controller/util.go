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
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func installFileIfChanged(src, dst string, perm os.FileMode) error {
	var srcBytes, dstBytes []byte

	src, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	srcBytes, err = os.ReadFile(src)
	if err != nil {
		return err
	}

	dstBytes, _ = os.ReadFile(dst)

	srcBytes = []byte(os.ExpandEnv(string(srcBytes)))

	if bytes.Compare(srcBytes, dstBytes) == 0 {
		log.Infof("file %s is not changed, skipping", dst)
		return nil
	}

	if err := backupFile(dst); err != nil {
		log.Warnf("Backup failed, %s", err)
	}

	log.Infof("install file %s to destination %s", src, dst)
	if err := os.WriteFile(dst, srcBytes, perm); err != nil {
		return err
	}

	return os.Chown(dst, 0, 0)
}

func backupFile(src string) error {
	log.Infof("backup %s file", src)

	if _, err := os.Stat(src); err != nil {
		return err
	}

	backupDir := filepath.Join(deckhousePath, "backup", config.ConfigurationChecksum)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	return copy.Copy(src, backupDir+src)
}

func removeFile(src string) error {
	log.Infof("remove %s file", src)
	if err := backupFile(src); err != nil {
		return err
	}
	return os.Remove(src)
}

func removeDirectory(dir string) error {
	walkDirFunc := func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		return removeFile(path)
	}

	err := filepath.WalkDir(dir, walkDirFunc)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func removeOrphanFiles() error {
	srcDir := filepath.Join(deckhousePath, "kubeadm", "patches")
	log.Infof("phase: remove orphan files from dir %s", srcDir)

	walkDirFunc := func(path string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() {
			return nil
		}

		switch _, file := filepath.Split(path); file {
		case "kube-apiserver.yaml":
			return nil
		case "etcd.yaml":
			return nil
		case "kube-controller-manager.yaml":
			return nil
		case "kube-scheduler.yaml":
			return nil
		default:
			return removeFile(path)
		}
	}

	return filepath.WalkDir(srcDir, walkDirFunc)
}

func kubeadm() string {
	return fmt.Sprintf("/usr/local/bin/kubeadm-%s", config.KubernetesVersion)
}

func stringSlicesEqual(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)

	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func waitImageHolderContainers() error {
	for {
		log.Info("phase: waiting for all image-holder containers will be ready")
		pod, err := config.K8sClient.CoreV1().Pods(namespace).Get(context.TODO(), config.MyPodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		isReady := true
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == "control-plane-manager" {
				continue
			}
			if !container.Ready {
				isReady = false
				break
			}
		}

		if isReady {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
}

func checkEtcdManifest() error {
	etcdManifestPath := filepath.Join(manifestsPath, "etcd.yaml")
	log.Infof("phase: check etcd manifest %s", etcdManifestPath)

	if _, err := os.Stat(etcdManifestPath); err != nil {
		log.Warnf("etcd manifest %s absent", etcdManifestPath)
		return nil
	}

	content, err := os.ReadFile(etcdManifestPath)
	if err != nil {
		return err
	}

	var pod v1.Pod

	if err := yaml.Unmarshal(content, pod); err != nil {
		return err
	}

	found := false
	for _, arg := range pod.Spec.Containers[0].Command {
		if !strings.HasPrefix(arg, "--advertise-client-urls=https://") {
			continue
		}
		if ip := strings.TrimSuffix(strings.TrimPrefix(arg, "--advertise-client-urls=https://"), ":2379"); ip != config.MyIP {
			return fmt.Errorf("etcd is not supposed to change advertise address from %s to %s, please verify node's InternalIP", ip, config.MyIP)
		}
		found = true
		break
	}

	if !found {
		return fmt.Errorf("cannot find --advertise-client-urls submatch in etcd manifest %s", etcdManifestPath)
	}

	found = false
	for _, arg := range pod.Spec.Containers[0].Command {
		if !strings.HasPrefix(arg, "--name=") {
			continue
		}
		if name := strings.TrimPrefix(arg, "--name="); name != config.NodeName {
			return fmt.Errorf("etcd is not supposed to change its name from %s to %s, please verify node's hostname", name, config.NodeName)
		}
		found = true
		break
	}

	if !found {
		return fmt.Errorf("cannot find --name submatch in etcd manifest %s", etcdManifestPath)
	}

	found = false
	for _, arg := range pod.Spec.Containers[0].Command {
		if !strings.HasPrefix(arg, "--data-dir=") {
			continue
		}
		if name := strings.TrimPrefix(arg, "--data-dir="); name != "/var/lib/etcd" {
			return fmt.Errorf("etcd is not supposed to change data-dir from %s to /var/lib/etcd, please verify current --data-dir", name)
		}
		found = true
		break
	}
	if !found {
		return fmt.Errorf("cannot find --data-dir submatch in etcd manifest %s", etcdManifestPath)
	}

	return nil
}

func checkKubeletConfig() error {
	kubeletPath := filepath.Join(kubernetesConfigPath, "kubelet.conf")
	log.Infof("phase: check kubelet config %s", kubeletPath)

	if _, err := os.Stat(kubeletPath); err != nil {
		// kubelet manifest does not exist, may be first run
		return fmt.Errorf("kubelet config does not exist in %s", kubeletPath)
	}

	content, err := os.ReadFile(kubeletPath)
	if err != nil {
		return err
	}

	res := &KubeConfigValue{}
	err = yaml.Unmarshal(content, res)
	if err != nil {
		return err
	}

	if res.Clusters[0].Cluster.Server == "https://127.0.0.1:6445" {
		return nil
	}

	return fmt.Errorf("cannot find server: https://127.0.0.1:6445 in kubelet config %s, kubelet should be configured "+
		"to access apiserver via kube-api-proxy (through https://127.0.0.1:6445), probably node is not managed by node-manager", kubeletPath)
}

func installKubeadmConfig() error {
	log.Info("phase: install kubeadm configuration")
	if err := os.MkdirAll(filepath.Join(deckhousePath, "kubeadm", "patches"), 0755); err != nil {
		return err
	}

	if err := installFileIfChanged(filepath.Join(configPath, "kubeadm-config.yaml"), filepath.Join(deckhousePath, "kubeadm", "config.yaml"), 0644); err != nil {
		return err
	}
	for _, component := range []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
		if err := installFileIfChanged(filepath.Join(configPath, component+".yaml.tpl"), filepath.Join(deckhousePath, "kubeadm", "patches", component+".yaml"), 0644); err != nil {
			return err
		}
	}
	return nil
}

func removeOldBackups() error {
	backupPath := filepath.Join(deckhousePath, "backup")
	log.Info("remove backups older than 5")
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return err
		}
		files = append(files, info)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})

	if len(files) <= 5 {
		return nil
	}
	for _, f := range files[5:] {
		log.Infof("removing old backup dir %s", f.Name())
		if err := os.RemoveAll(filepath.Join(backupPath, f.Name())); err != nil {
			return err
		}
	}
	return nil
}
