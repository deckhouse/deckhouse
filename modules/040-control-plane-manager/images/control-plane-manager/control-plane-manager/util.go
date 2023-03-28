package main

import (
	"bytes"
	"os"

	"github.com/Masterminds/semver/v3"
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

	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	dstBytes, _ = os.ReadFile(src)
	if err != nil {
		return err
	}

	srcBytes = []byte(os.ExpandEnv(string(srcBytes)))

	if bytes.Compare(srcBytes, dstBytes) == 0 {
		log.Infof("file %s is not changed, skipping", src)
		return nil
	}

	log.Infof("install file %s to destination %s", src, dst)
	err = os.WriteFile(dst, srcBytes, perm)
	if err != nil {
		return err
	}
	return os.Chown(dst, 0, 0)
}
