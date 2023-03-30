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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func renewKubeconfigs() error {
	for _, v := range []string{"admin", "controller-manager", "kube-scheduler"} {
		if err := renewKubeconfig(v); err != nil {
			return err
		}
	}
	return nil
}

func renewKubeconfig(componentName string) error {
	path := filepath.Join(kubernetesConfigPath, componentName+".conf")
	log.Infof("generate or renew %s kubeconfig", path)
	if _, err := os.Stat(path); err == nil && configurationChecksum != lastAppliedConfigurationChecksum {
		var remove bool
		log.Infof("configuration has changed since last kubeconfig generation (last applied checksum %s, configuration checksum %s), verifying kubeconfig", lastAppliedConfigurationChecksum, configurationChecksum)
		if err := prepareKubeconfig(componentName, true); err != nil {
			return err
		}

		currentKubeconfig, err := loadKubeconfig(path)
		if err != nil {
			return err
		}
		tmpKubeconfig, err := loadKubeconfig(filepath.Join("/tmp", configurationChecksum, path))
		if err != nil {
			return err
		}

		if currentKubeconfig.Clusters[0].Cluster.Server != tmpKubeconfig.Clusters[0].Cluster.Server {
			log.Infof("kubeconfig %s address field changed", path)
			remove = true
		}

		var certData []byte
		fmt.Println("%v",  currentKubeconfig)
		if _, err := base64.StdEncoding.Decode(certData, []byte(currentKubeconfig.Users[0].User.ClientCertificateData)); err != nil {
			return err
		}

		cert, err := x509.ParseCertificate(certData)
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
	c := &exec.Cmd{}
	if isTemp {
		tmpPath := filepath.Join("/tmp", configurationChecksum)
		c = exec.Command(kubeadm(), "init", "phase", "kubeconfig", componentName, "--config", deckhousePath+"/kubeadm/config.yaml", "--rootfs", tmpPath)
	} else {
		c = exec.Command(kubeadm(), "init", "phase", "kubeconfig", componentName, "--config", deckhousePath+"/kubeadm/config.yaml")
	}
	out, err := c.CombinedOutput()
	log.Infof("%s", out)
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
