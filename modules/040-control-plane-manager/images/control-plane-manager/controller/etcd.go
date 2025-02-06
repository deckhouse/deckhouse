package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func etcdJoinConverge() error {
	// kubeadm -v=5 join phase control-plane-join etcd --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
	args := []string{"-v=5", "join", "phase", "control-plane-join", "etcd", "--config", deckhousePath + "/kubeadm/config.yaml"}
	c := exec.Command(kubeadmPath, args...)
	out, err := c.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Infof("%s", s)
	}

	isLearner, err := checkIfNodeIsLearner()
	if err != nil {
		return fmt.Errorf("failed to check learner status: %v", err)
	}
	if !isLearner {
		log.Infof("Etcd member for node %q is not in learner mode. Join succeeded.", config.NodeName)
		return nil
	}
	if isLearner {
		return fmt.Errorf("kubeadm join has been executed but the node is still in learner phase: %v", err)
	}

	return err
}

func checkIfNodeIsLearner() (bool, error) {
	cli, err := newEtcdClient()
	if err != nil {
		return false, fmt.Errorf("failed to create etcd client: %v", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := cli.MemberList(ctx)
	if err != nil {
		return false, fmt.Errorf("MemberList request failed: %v", err)
	}

	for _, m := range resp.Members {
		if m.Name == config.NodeName {
			log.Infof("etcd member: ID=%d Name=%q PeerURLs=%v IsLearner=%v",
				m.ID, m.Name, m.PeerURLs, m.IsLearner)
			return m.IsLearner, nil
		}
	}

	return false, nil
}

func newEtcdClient() (*clientv3.Client, error) {
	caCertPath := "/etc/kubernetes/pki/etcd/ca.crt"
	certPath := "/etc/kubernetes/pki/etcd/ca.crt"
	keyPath := "/etc/kubernetes/pki/etcd/ca.key"

	tlsConfig, err := buildTLSConfig(caCertPath, certPath, keyPath)
	if err != nil {
		return nil, err
	}

	cfg := clientv3.Config{
		Endpoints:   []string{"https://127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	}

	return clientv3.New(cfg)
}

func buildTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caCertBytes, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read CA file %q: %v", caFile, err)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot load client cert/key %q, %q: %v", certFile, keyFile, err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
	}

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}
