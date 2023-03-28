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

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

var (
	myPodName         string
	kubernetesVersion string
	nodeName          string
	myIP              string
	k8sClient         *kubernetes.Clientset
	quit              = make(chan struct{})
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

func annotateNode() error {
	log.Infof("annotate node %s with annotation '%s'", nodeName, waitingApprovalAnnotation)
	node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if _, ok := node.Annotations[approvedAnnotation]; ok {
		// node already approved, no need to annotate
		log.Infof("node %s already approved by annotation '%s', no need to annotate", nodeName, approvedAnnotation)
		return nil
	}

	node.Annotations[waitingApprovalAnnotation] = ""

	_, err = k8sClient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

func waitNodeApproval() error {
	for i := 0; i < maxRetries; i++ {
		log.Infof("waiting for '%s' annotation on our node %s", approvedAnnotation, nodeName)
		node, err := k8sClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if _, ok := node.Annotations[approvedAnnotation]; ok {
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return errors.Errorf("can't get annotation '%s' from our node %s", approvedAnnotation, nodeName)
}

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

func checkKubernetesVersion() error {
	log.Info("check desired kubernetes version %s", kubernetesVersion)
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
	return errors.Errorf("kubernetes version '%s' is not allowed", kubernetesVersion)

}

func checkEtcdManifest() error {
	etcdManifestPath := filepath.Join(manifestsPath, "etcd.yaml")
	log.Info("check etcd manifest '%s'", etcdManifestPath)

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
		return errors.New("cannot find '--advertise-client-urls' submatch in etcd manifest")
	}
	if string(res[1]) != myIP {
		return errors.Errorf("etcd is not supposed to change advertise address from '%s' to '%s'. Verify Node's InternalIP.", res[1], myIP)
	}

	re = regexp.MustCompile(`--name=(.+)`)
	res = re.FindSubmatch(content)
	if len(res) < 2 {
		return errors.New("cannot find '--name' submatch in etcd manifest")
	}
	if string(res[1]) != nodeName {
		return errors.Errorf("etcd is not supposed to change its name from '%s' to '%s'. Verify Node's hostname.", res[1], nodeName)
	}

	re = regexp.MustCompile(`--data-dir=(.+)`)
	res = re.FindSubmatch(content)
	if len(res) < 2 {
		return errors.New("cannot find '--data-dir' submatch in etcd manifest")
	}
	if string(res[1]) != "/var/lib/etcd" {
		return errors.Errorf("etcd is not supposed to change data-dir from '%s' to '/var/lib/etcd'. Verify current '--data-dir'.", res[1])
	}

	return nil
}

func checkKubeletConfig() error {
	kubeletPath := filepath.Join(kubernetesConfigPath, "kubelet.conf")
	log.Info("check kubelet config '%s'", kubeletPath)

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

	return errors.Errorf("cannot find 'server: https://127.0.0.1:6445' in kubelet config '%s'", kubeletPath)
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

	// pause loop
	<-quit
}
