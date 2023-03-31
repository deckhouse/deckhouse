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
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func waitImageHolderContainers() error {
	for {
		log.Info("waiting for all image-holder containers will be ready")
		pod, err := k8sClient.CoreV1().Pods(namespace).Get(context.TODO(), myPodName, metav1.GetOptions{})
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
	log.Infof("check etcd manifest %s", etcdManifestPath)

	if _, err := os.Stat(etcdManifestPath); err != nil {
		// etcd manifest does not exist, may be first run
		return nil
	}

	content, err := os.ReadFile(etcdManifestPath)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`--advertise-client-urls=https://(.+):2379`)
	res := re.FindSubmatch(content)
	if len(res) < 2 {
		return errors.New("cannot find --advertise-client-urls submatch in etcd manifest")
	}
	if string(res[1]) != myIP {
		return errors.Errorf("etcd is not supposed to change advertise address from %s to %s, please verify node's InternalIP", res[1], myIP)
	}

	re = regexp.MustCompile(`--name=(.+)`)
	res = re.FindSubmatch(content)
	if len(res) < 2 {
		return errors.New("cannot find --name submatch in etcd manifest")
	}
	if string(res[1]) != nodeName {
		return errors.Errorf("etcd is not supposed to change its name from %s to %s, please verify node's hostname", res[1], nodeName)
	}

	re = regexp.MustCompile(`--data-dir=(.+)`)
	res = re.FindSubmatch(content)
	if len(res) < 2 {
		return errors.New("cannot find --data-dir submatch in etcd manifest")
	}
	if string(res[1]) != "/var/lib/etcd" {
		return errors.Errorf("etcd is not supposed to change data-dir from %s to /var/lib/etcd, please verify current --data-dir", res[1])
	}

	return nil
}

func checkKubeletConfig() error {
	kubeletPath := filepath.Join(kubernetesConfigPath, "kubelet.conf")
	log.Infof("check kubelet config %s", kubeletPath)

	if _, err := os.Stat(kubeletPath); err != nil {
		// kubelet manifest does not exist, may be first run
		return errors.Errorf("kubelet config does not exist in %s", kubeletPath)
	}

	content, err := os.ReadFile(kubeletPath)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`server: https://127.0.0.1:6445`)
	if re.Match(content) {
		return nil
	}

	return errors.Errorf("cannot find server: https://127.0.0.1:6445 in kubelet config %s, kubelet should be configured "+
		"to access apiserver via kube-api-proxy (through https://127.0.0.1:6445), probably node is not managed by node-manager", kubeletPath)
}

func installKubeadmConfig() error {
	log.Info("install kubeadm configuration")
	kubeadmDir := filepath.Join(deckhousePath, "kubeadm")
	patchesDir := filepath.Join(kubeadmDir, "patches")
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		return err
	}

	if err := installFileIfChanged(filepath.Join(configPath, "kubeadm-config.yaml"), filepath.Join(kubeadmDir, "config.yaml"), 0644); err != nil {
		return err
	}
	for _, component := range []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
		if err := installFileIfChanged(filepath.Join(configPath, component+".yaml.tpl"), filepath.Join(patchesDir, component+".yaml"), 0644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	if err := readEnvs(); err != nil {
		log.Fatal(err)
	}

	if err := checkKubernetesVersion(); err != nil {
		log.Fatal(err)
	}

	if err := newClient(); err != nil {
		log.Fatal(err)
	}

	if err := calculateConfigurationChecksum(); err != nil {
		log.Fatal(err)
	}

	// At the first run there is can be error
	if err := getLastAppliedConfigurationChecksum(); err != nil {
		log.Errorf("%s, it is normal on the first run", err)
	}

	// At the first run there is can be error
	if err := removeOrphanFiles(); err != nil {
		log.Errorf("%s, it is normal on the first run", err)
	}

	if err := annotateNode(); err != nil {
		log.Fatal(err)
	}

	if err := waitNodeApproval(); err != nil {
		log.Fatal(err)
	}

	if err := waitImageHolderContainers(); err != nil {
		log.Fatal(err)
	}

	if err := checkEtcdManifest(); err != nil {
		log.Fatal(err)
	}

	if err := checkKubeletConfig(); err != nil {
		log.Fatal(err)
	}

	if err := installKubeadmConfig(); err != nil {
		log.Fatal(err)
	}

	// certificates config phase
	if err := installBasePKIfiles(); err != nil {
		log.Fatal(err)
	}

	if err := fillTmpDirWithPKIData(); err != nil {
		log.Fatal(err)
	}

	if err := renewCertificates(); err != nil {
		log.Fatal(err)
	}

	// kubeconfig config phase
	if err := renewKubeconfigs(); err != nil {
		log.Fatal(err)
	}

	if err := updateRootKubeconfig(); err != nil {
		log.Fatal(err)
	}

	// converge phase
	if err := installExtraFiles(); err != nil {
		log.Fatal(err)
	}

	if err := convergeComponents(); err != nil {
		log.Fatal(err)
	}

	// pause loop
	<-quit
}
