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
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
)

func installBasePKIfiles() error {
	log.Info("install base pki files")
	if err := os.MkdirAll("/etc/kubernetes/pki/etcd", 0755); err != nil {
		return err
	}

	for _, f := range []string{"ca.crt", "front-proxy-ca.crt"} {
		if err := installFileIfChanged(filepath.Join(pkiPath, f), filepath.Join(kubernetesConfigPath, "pki", f), 0644); err != nil {
			return err
		}
	}

	for _, f := range []string{"ca.key", "sa.pub", "sa.key", "front-proxy-ca.key"} {
		if err := installFileIfChanged(filepath.Join(pkiPath, f), filepath.Join(kubernetesConfigPath, "pki", f), 0600); err != nil {
			return err
		}
	}

	for _, f := range []string{"ca.key", "sa.pub", "sa.key", "front-proxy-ca.key"} {
		if err := installFileIfChanged(filepath.Join(pkiPath, f), filepath.Join(kubernetesConfigPath, "pki", f), 0600); err != nil {
			return err
		}
	}

	if err := installFileIfChanged(filepath.Join(pkiPath, "etcd-ca.crt"), filepath.Join(kubernetesConfigPath, "pki", "etcd", "ca.crt"), 0644); err != nil {
		return err
	}

	if err := installFileIfChanged(filepath.Join(pkiPath, "etcd-ca.key"), filepath.Join(kubernetesConfigPath, "pki", "etcd", "ca.key"), 0600); err != nil {
		return err
	}

	return nil
}

func renewCertificates() error {
	components := make(map[string]string, 7)
	components["apiserver"] = "apiserver"
	components["apiserver-kubelet-client"] = "apiserver-kubelet-client"
	components["apiserver-etcd-client"] = "apiserver-etcd-client"
	components["front-proxy-client"] = "front-proxy-client"
	components["etcd-server"] = "etcd/server"
	components["etcd-peer"] = "etcd/peer"
	components["etcd-healthcheck-client"] = "etcd/healthcheck-client"
	for k, v := range components {
		if err := renewCertificate(k, v); err != nil {
			return err
		}
	}
	return nil
}

func renewCertificate(componentName, f string) error {
	path := filepath.Join(kubernetesConfigPath, "pki", f+".crt")
	log.Infof("generate or renew %s certificate %s", componentName, path)

	if _, err := os.Stat(path); err == nil && configurationChecksum != lastAppliedConfigurationChecksum {
		var remove bool
		log.Infof("configuration has changed since last certificate generation (last applied checksum %s, configuration checksum %s), verifying certificate", lastAppliedConfigurationChecksum, configurationChecksum)
		if err := prepareCerts(componentName, true); err != nil {
			return err
		}

		tmpPath := filepath.Join("/tmp", configurationChecksum)
		oldCert, err := loadCert(path)
		if err != nil {
			return err
		}

		tmpCert, err := loadCert(filepath.Join(tmpPath, path))
		if err != nil {
			return err
		}

		if !certificateSubjectAndSansIsEqual(oldCert, tmpCert) {
			log.Infof("certificate %s subject or sans has been changed", path)
			remove = true
		}

		if certificateExpiresSoon(oldCert, 30*24*time.Hour) {
			log.Infof("certificate %s is expiring in less than 30 days", path)
			remove = true
		}

		keyPath := filepath.Join(kubernetesConfigPath, "pki", f+".key")
		if _, err := os.Stat(keyPath); err != nil {
			log.Infof("certificate %s exists, but no appropriate key found")
			remove = true
		}

		if remove {
			if err := removeFile(path); err != nil {
				log.Error(err)
			}
			if err := removeFile(keyPath); err != nil {
				log.Error(err)
			}
		}
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	}
	// regenerate certificate
	return prepareCerts(componentName, false)
}

func certificateSubjectAndSansIsEqual(a, b *x509.Certificate) bool {

	aCertSans := a.DNSNames
	for _, ip := range a.IPAddresses {
		aCertSans = append(aCertSans, ip.String())
	}

	bCertSans := b.DNSNames
	for _, ip := range b.IPAddresses {
		bCertSans = append(bCertSans, ip.String())
	}

	return reflect.DeepEqual(a.Subject, b.Subject) &&
		stringSlicesEqual(aCertSans, bCertSans)
}

func fillTmpDirWithPKIData() error {
	tmpPath := filepath.Join("/tmp", configurationChecksum)

	if err := os.RemoveAll(tmpPath); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(tmpPath, kubernetesConfigPath, "pki", "etcd"), 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(tmpPath, deckhousePath), 0755); err != nil {
		return err
	}

	if err := copy.Copy(deckhousePath, filepath.Join(tmpPath, deckhousePath)); err != nil {
		return err
	}

	for _, file := range []string{"front-proxy-ca.crt", "front-proxy-ca.key", "ca.crt", "ca.key", "etcd/ca.crt", "etcd/ca.key"} {
		if err := copy.Copy(filepath.Join(kubernetesConfigPath, "pki", file), filepath.Join(tmpPath, kubernetesConfigPath, "pki", file)); err != nil {
			return err
		}
	}
	return nil
}

func loadCert(path string) (*x509.Certificate, error) {
	r, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(r)
	return x509.ParseCertificate(block.Bytes)
}

func certificateExpiresSoon(c *x509.Certificate, durationLeft time.Duration) bool {
	return time.Until(c.NotAfter) < durationLeft
}

func prepareCerts(componentName string, isTemp bool) error {
	c := &exec.Cmd{}
	if isTemp {
		tmpPath := filepath.Join("/tmp", configurationChecksum)
		c = exec.Command(kubeadm(), "init", "phase", "certs", componentName, "--config", deckhousePath+"/kubeadm/config.yaml", "--rootfs", tmpPath)
	} else {
		c = exec.Command(kubeadm(), "init", "phase", "certs", componentName, "--config", deckhousePath+"/kubeadm/config.yaml")
	}
	out, err := c.CombinedOutput()
	log.Infof("%s", out)
	return err
}
