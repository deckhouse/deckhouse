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
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	waitingApprovalAnnotation = `control-plane-manager.deckhouse.io/waiting-for-approval`
	approvedAnnotation        = `control-plane-manager.deckhouse.io/approved`
	maxRetries                = 180
	namespace                 = `kube-system`
	kubernetesConfigPath      = `/etc/kubernetes`
	manifestsPath             = kubernetesConfigPath + `/manifests`
	deckhousePath             = kubernetesConfigPath + `/deckhouse`
	configPath                = `/config`
	pkiPath                   = `/pki`
	kubernetesPkiPath         = kubernetesConfigPath + `/pki`
	kubeadmPath               = "/kubeadm"
)

type Config struct {
	MyPodName                        string
	KubernetesVersion                string
	NodeName                         string
	MyIP                             string
	K8sClient                        *kubernetes.Clientset
	ExitChannel                      chan struct{}
	ConfigurationChecksum            string
	LastAppliedConfigurationChecksum string
	TmpPath                          string
	AllowedKubernetesVersions        string
}

var (
	config                     *Config
	controlPlaneManagerIsReady bool
	server                     *http.Server
	nowTime                    = time.Now()
)

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
	config.TmpPath = filepath.Join("/tmp", config.ConfigurationChecksum)
	config.ExitChannel = make(chan struct{}, 1)
	return config, nil
}

func (c *Config) readEnvs() error {
	var (
		ok  bool
		err error
	)
	if c.MyPodName, ok = os.LookupEnv("MY_POD_NAME"); !ok || len(c.MyPodName) == 0 {
		return errors.New("MY_POD_NAME env should be set")
	}

	if c.MyIP, ok = os.LookupEnv("MY_IP"); !ok || len(c.MyIP) == 0 {
		return errors.New("MY_IP env should be set")
	}

	if c.KubernetesVersion, ok = os.LookupEnv("KUBERNETES_VERSION"); !ok || len(c.KubernetesVersion) == 0 {
		return errors.New("KUBERNETES_VERSION env should be set")
	}

	if c.AllowedKubernetesVersions, ok = os.LookupEnv("ALLOWED_KUBERNETES_VERSIONS"); !ok || len(c.AllowedKubernetesVersions) == 0 {
		return errors.New("ALLOWED_KUBERNETES_VERSIONS env should be set")
	}

	if err := c.checkKubernetesVersion(); err != nil {
		return err
	}

	c.NodeName, err = os.Hostname()
	if err != nil {
		return err
	}
	if c.NodeName == "" {
		return errors.New("node name should be set")
	}
	return nil
}

func (c *Config) newClient() error {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	c.K8sClient, err = kubernetes.NewForConfig(config)
	return err
}

func (c *Config) checkKubernetesVersion() error {
	log.Infof("check desired kubernetes version %s against allowed kubernetes version list: %s", c.KubernetesVersion, c.AllowedKubernetesVersions)

	for _, v := range strings.Split(c.AllowedKubernetesVersions, ",") {
		if c.KubernetesVersion == v {
			return nil
		}
	}
	return fmt.Errorf("kubernetes version %s is not supported", c.KubernetesVersion)
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
	path := filepath.Join(deckhousePath, "last_applied_configuration_checksum")
	// it is normal if last applied configuration checksum file is absent on the first run
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	srcBytes, err := os.ReadFile(path)
	c.LastAppliedConfigurationChecksum = strings.Trim(string(srcBytes), "\n")
	return err
}

func (c *Config) writeLastAppliedConfigurationChecksum() error {
	return os.WriteFile(filepath.Join(deckhousePath, "last_applied_configuration_checksum"), []byte(c.LastAppliedConfigurationChecksum), 0644)
}
