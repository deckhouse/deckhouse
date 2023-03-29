package main

import (
	"os"
	"path/filepath"

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
