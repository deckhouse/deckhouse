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
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	waitingApprovalAnnotation          = `control-plane-manager.deckhouse.io/waiting-for-approval`
	approvedAnnotation                 = `control-plane-manager.deckhouse.io/approved`
	maxRetries                         = 42
	namespace                          = `kube-system`
	minimalKubernetesVersionConstraint = `>= 1.22`
	maximalKubernetesVersionConstraint = `< 1.27`
	kubernetesConfigPath               = `/etc/kubernetes`
	manifestsPath                      = kubernetesConfigPath + `/manifests`
	configPath                         = `/config`
	pkiPath                            = `/pki`
)

var (
	myPodName                        string
	kubernetesVersion                string
	nodeName                         string
	myIP                             string
	k8sClient                        *kubernetes.Clientset
	quit                             = make(chan struct{})
	configurationChecksum            string
	lastAppliedConfigurationChecksum string
)

func readEnvs() error {
	myPodName = os.Getenv("MY_POD_NAME")
	if myPodName == "" {
		return errors.New("MY_POD_NAME env should be set")
	}

	myIP = os.Getenv("MY_IP")
	if myIP == "" {
		return errors.New("MY_IP env should be set")
	}

	kubernetesVersion = os.Getenv("KUBERNETES_VERSION")
	if kubernetesVersion == "" {
		return errors.New("KUBERNETES_VERSION env should be set")
	}

	// get hostname
	h, err := os.Hostname()
	if err != nil {
		return err
	}
	if h == "" {
		return errors.New("node name should be set")
	}
	nodeName = h
	return nil
}

func newClient() error {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k8sClient, err = kubernetes.NewForConfig(config)
	return err
}

func checkKubernetesVersion() error {
	log.Infof("check desired kubernetes version %s", kubernetesVersion)
	minimalConstraint, err := semver.NewConstraint(minimalKubernetesVersionConstraint)
	if err != nil {
		log.Fatal(err)
	}

	maximalConstraint, err := semver.NewConstraint(maximalKubernetesVersionConstraint)
	if err != nil {
		log.Fatal(err)
	}

	v := semver.MustParse(kubernetesVersion)
	if minimalConstraint.Check(v) && maximalConstraint.Check(v) {
		return nil
	}
	return errors.Errorf("kubernetes version %s is not allowed", kubernetesVersion)
}

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
		return err
	}

	log.Infof("install file %s to destination %s", src, dst)
	if err := os.WriteFile(dst, srcBytes, perm); err != nil {
		return err
	}

	return os.Chown(dst, 0, 0)
}

func calculateConfigurationChecksum() error {
	h := sha256.New()
	f, err := os.Open(os.Args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	// For tests
	configDir := configPath
	if env := os.Getenv("TESTS_CONFIG_PATH"); env != "" {
		configDir = env
	}

	dirEntries, err := os.ReadDir(configDir)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		path, err := filepath.EvalSymlinks(filepath.Join(configDir, entry.Name()))
		if err != nil {
			return err
		}

		fileInfo, err := os.Stat(path)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(h, f); err != nil {
			return err
		}
	}
	configurationChecksum = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

func getLastAppliedConfigurationChecksum() error {
	var srcBytes []byte
	srcBytes, err := os.ReadFile(filepath.Join(kubernetesConfigPath, "deckhouse", "last_applied_configuration_checksum"))
	lastAppliedConfigurationChecksum = string(srcBytes)
	return err
}

func backupFile(src string) error {
	log.Infof("backup %s file", src)

	if _, err := os.Stat(src); err != nil {
		return err
	}

	backupDir := filepath.Join(kubernetesConfigPath, "deckhouse", "backup", configurationChecksum)

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

func removeOrphanFiles(srcDir string) error {
	log.Infof("remove orphan files from dir %s", srcDir)

	walkFunc := func(path string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
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
	return filepath.Walk(srcDir, walkFunc)
}
