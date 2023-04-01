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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func renewKubeconfigs() error {
	log.Info("phase: renew kubeconfigs")
	for _, v := range []string{"admin", "controller-manager", "scheduler"} {
		if err := renewKubeconfig(v); err != nil {
			return err
		}
	}
	return nil
}

func renewKubeconfig(componentName string) error {
	path := filepath.Join(kubernetesConfigPath, componentName+".conf")
	log.Infof("generate or renew %s kubeconfig", path)
	if _, err := os.Stat(path); err == nil && config.ConfigurationChecksum != config.LastAppliedConfigurationChecksum {
		var remove bool
		log.Infof("configuration has changed since last kubeconfig generation (last applied checksum %s, configuration checksum %s), verifying kubeconfig", config.LastAppliedConfigurationChecksum, config.ConfigurationChecksum)
		if err := prepareKubeconfig(componentName, true); err != nil {
			return err
		}

		currentKubeconfig, err := loadKubeconfig(path)
		if err != nil {
			return err
		}
		tmpKubeconfig, err := loadKubeconfig(filepath.Join(config.TmpPath, path))
		if err != nil {
			return err
		}

		if currentKubeconfig.Clusters[0].Cluster.Server != tmpKubeconfig.Clusters[0].Cluster.Server {
			log.Infof("kubeconfig %s address field changed", path)
			remove = true
		}

		certData, err := base64.StdEncoding.DecodeString(currentKubeconfig.Users[0].User.ClientCertificateData)
		if err != nil {
			return err
		}
		block, _ := pem.Decode(certData)
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return err
		}

		if certificateExpiresSoon(cert, 30*24*time.Hour) {
			log.Infof("client certificate in kubeconfig %s is expiring in less than 30 days", path)
			remove = true
		}

		if remove {
			if err := removeFile(path); err != nil {
				log.Error(err)
			}
		}
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// regenerate kubeconfig
	log.Infof("generate new kubeconfig %s", path)
	return prepareKubeconfig(componentName, false)
}

func prepareKubeconfig(componentName string, isTemp bool) error {
	args := []string{"init", "phase", "kubeconfig", componentName, "--config", deckhousePath + "/kubeadm/config.yaml"}
	if isTemp {
		args = append(args, "--rootfs", config.TmpPath)
	}
	c := exec.Command(kubeadm(), args...)
	out, err := c.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Infof("%s", s)
	}
	return err
}

func loadKubeconfig(path string) (*KubeConfigValue, error) {
	res := &KubeConfigValue{}
	r, err := os.ReadFile(path)
	if err != nil {
		return res, err
	}

	err = yaml.Unmarshal(r, res)
	return res, err
}

func updateRootKubeconfig() error {
	path := "/root/.kube/config"
	originalPath := filepath.Join(kubernetesConfigPath, "admin.conf")
	log.Infof("update root user kubeconfig (%s)", path)
	if _, err := os.Stat(path); err == nil {
		p, err := filepath.EvalSymlinks(path)
		if p == originalPath && err == nil {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	return os.Symlink(originalPath, path)
}
