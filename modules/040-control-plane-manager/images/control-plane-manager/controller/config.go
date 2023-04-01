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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
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
	deckhousePath                      = kubernetesConfigPath + `/deckhouse`
	configPath                         = `/config`
	pkiPath                            = `/pki`
	kubernetesPkiPath                  = kubernetesConfigPath + `/pki`
)

type Config struct {
	MyPodName                        string
	KubernetesVersion                string
	NodeName                         string
	MyIP                             string
	K8sClient                        *kubernetes.Clientset
	Quit                             chan struct{}
	ConfigurationChecksum            string
	LastAppliedConfigurationChecksum string
	TmpPath                          string
}

func NewConfig() (*Config, error) {
	config := &Config{}
	if err := config.readEnvs(); err != nil {
		return config, err
	}
	if err := config.newClient(); err != nil {
		return config, err
	}
	if err := config.calculateConfigurationChecksum(); err != nil {
		return config, err
	}
	if err := config.getLastAppliedConfigurationChecksum(); err != nil {
		return config, err
	}
	config.Quit = make(chan struct{})
	config.TmpPath = filepath.Join("/tmp", config.ConfigurationChecksum)
	return config, nil
}

func (c *Config) readEnvs() error {
	c.MyPodName = os.Getenv("MY_POD_NAME")
	if c.MyPodName == "" {
		return errors.New("MY_POD_NAME env should be set")
	}

	c.MyIP = os.Getenv("MY_IP")
	if c.MyIP == "" {
		return errors.New("MY_IP env should be set")
	}

	c.KubernetesVersion = os.Getenv("KUBERNETES_VERSION")
	if c.KubernetesVersion == "" {
		return errors.New("KUBERNETES_VERSION env should be set")
	}

	if err := checkKubernetesVersion(c.KubernetesVersion); err != nil {
		return err
	}

	// get hostname
	h, err := os.Hostname()
	if err != nil {
		return err
	}
	if h == "" {
		return errors.New("node name should be set")
	}
	c.NodeName = h
	return nil
}

func (c *Config) newClient() error {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	c.K8sClient = k8sClient
	return nil
}

func checkKubernetesVersion(kubernetesVersion string) error {
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

func (c *Config) calculateConfigurationChecksum() error {
	h := sha256.New()
	f, err := os.Open(os.Args[0])
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	// For tests
	configDir := configPath
	if env := os.Getenv("D8_TESTS"); env == "yes" {
		configDir = "testdata/config"
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
		//goland:noinspection GoUnhandledErrorResult
		defer f.Close()

		if _, err := io.Copy(h, f); err != nil {
			return err
		}
	}
	c.ConfigurationChecksum = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

func (c *Config) getLastAppliedConfigurationChecksum() error {
	var srcBytes []byte
	srcBytes, err := os.ReadFile(filepath.Join(deckhousePath, "last_applied_configuration_checksum"))
	c.LastAppliedConfigurationChecksum = strings.Trim(string(srcBytes), "\n")
	return err
}
