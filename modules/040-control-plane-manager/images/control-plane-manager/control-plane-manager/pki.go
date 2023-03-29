package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
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

func kubeadm() string {
	return fmt.Sprintf("/usr/local/bin/kubeadm-%s", kubernetesVersion)
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
		log.Info("configuration has changed since last certificate generation, verifying certificate")
		changed, err := certificateSubjectAndSansIsChanged(componentName, path)
		if err != nil {
			return err
		}
		if changed {
			log.Infof("certificate %s subject or sans has been changed", path)
			remove = true
		}
		if err != nil {
			return err
		}

		expires, err := certificateExpiresSoon(path, 30*24*time.Hour)
		if expires {
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

	if _, err := os.Stat(path); err != nil {
		// regenerate certificate
		log.Infof("generate certificate %s", path)
		c := exec.Command(fmt.Sprintf("%s init phase certs %s --config %s/kubeadm/config.yaml", kubeadm(), componentName, deckhousePath))
		out, err := c.CombinedOutput()
		if err != nil {
			return err
		}
		log.Infof("%s", out)
	}

	return nil
}

func certificateSubjectAndSansIsChanged(componentName, path string) (bool, error) {
	// Generate tmp certificate and compare
	tmpPath := filepath.Join("/tmp", configurationChecksum)
	c := exec.Command(fmt.Sprintf("%s init phase certs %s --config %s/kubeadm/config.yaml --rootfs %s", kubeadm(), componentName, deckhousePath, tmpPath))
	out, err := c.CombinedOutput()
	if err != nil {
		return false, err
	}
	log.Infof("%s", out)

	oldCert, err := loadCert(path)
	if err != nil {
		return false, err
	}

	tmpCert, err := loadCert(filepath.Join(tmpPath, path))
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(oldCert.Subject, tmpCert.Subject) &&
		reflect.DeepEqual(oldCert.DNSNames, tmpCert.DNSNames) &&
		reflect.DeepEqual(oldCert.EmailAddresses, tmpCert.EmailAddresses) &&
		reflect.DeepEqual(oldCert.IPAddresses, tmpCert.IPAddresses) &&
		reflect.DeepEqual(oldCert.URIs, tmpCert.URIs), nil
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

func certificateExpiresSoon(path string, durationLeft time.Duration) (bool, error) {
	c, err := loadCert(path)
	if err != nil {
		return false, err
	}
	return time.Until(c.NotAfter) > durationLeft, nil
}
