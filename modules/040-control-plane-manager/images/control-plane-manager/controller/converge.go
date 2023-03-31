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
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

func installExtraFiles() error {
	dstDir := filepath.Join(deckhousePath, "extra-files")
	log.Infof("install extra files to %s", dstDir)

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
	if _, err := os.Stat(filepath.Join(manifestsPath, manifestsPath, componentName+".yaml")); err != nil {
		recreateConfig = true
	} else {
		equal, err := manifestChecksumIsEqual(componentName, checksum)
		if err != nil {
			return err
		}
		if !equal {
			recreateConfig = true
		}
	}

	if recreateConfig {
		log.Infof("generate new kubeconfig for %s", componentName)
		if err := prepareConverge(componentName, false); err != nil {
			return err
		}
	}

	return nil
}

func prepareConverge(componentName string, isTemp bool) error {
	args := []string{"init", "phase"}
	if componentName == "etcd" {
		args = append(args, "etcd", "local", componentName, "--config", deckhousePath+"/kubeadm/config.yaml")
	} else {
		args = append(args, "control-plane", strings.TrimPrefix(componentName, "kube-"), "--config", deckhousePath+"/kubeadm/config.yaml")
	}
	if isTemp {
		args = append(args, "--rootfs", filepath.Join("/tmp", configurationChecksum))
	}
	c := exec.Command(kubeadm(), args...)
	out, err := c.CombinedOutput()
	log.Infof("%s", out)
	return err
}

func calculateChecksum(componentName string) (string, error) {
	manifest, err := os.ReadFile(filepath.Join("/tmp", configurationChecksum, manifestsPath, componentName+".yaml"))
	if err != nil {
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

	sha256sumSlice := make([]string, len(filesSlice))
	i = 0
	for _, file := range filesSlice {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		sha256sum, err := calculateSha256(content)
		if err != nil {
			return "", err
		}
		sha256sumSlice[i] = sha256sum
		i++
	}

	sort.Strings(sha256sumSlice)
	for _, v := range sha256sumSlice {
		manifest = append(manifest, []byte(v)...)
	}

	sha256sum, err := calculateSha256(manifest)

	return sha256sum, nil
}

func calculateSha256(content []byte) (string, error) {
	h := sha256.New()
	if _, err := h.Write(content); err != nil {
		return "", err
	}
	return string(h.Sum(nil)), nil
}

func manifestChecksumIsEqual(componentName, checksum string) (bool, error) {
	content, err := os.ReadFile(filepath.Join(manifestsPath, manifestsPath, componentName+".yaml"))
	if err != nil {
		return false, err
	}
	return strings.Index(string(content), checksum) != -1, nil
}
