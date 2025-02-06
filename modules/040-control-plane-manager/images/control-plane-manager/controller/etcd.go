package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	etcdTimeout         = 2 * time.Second
	NotFoundID   uint64 = 0
	etcdEndpoint        = "https://127.0.0.1:2379"
)

// Exponential backoff for etcd operations (up to ~200 seconds)
var etcdBackoff = wait.Backoff{
	Steps:    18,
	Duration: 100 * time.Millisecond,
	Factor:   1.5,
	Jitter:   0.1,
}

type EtcdClient struct {
	client *clientv3.Client
	config *Config
}

func (c *EtcdClient) EtcdJoinConverge() error {
	// kubeadm -v=5 join phase control-plane-join etcd --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
	args := []string{"-v=5", "join", "phase", "control-plane-join", "etcd", "--config", deckhousePath + "/kubeadm/config.yaml"}
	cli := exec.Command(kubeadmPath, args...)
	out, err := cli.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Infof("%s", s)
	}
	return err
}

func (c *EtcdClient) CheckIfNodeIsLearner() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.MemberList(ctx)
	if err != nil {
		return NotFoundID, fmt.Errorf("MemberList request failed: %v", err)
	}

	for _, m := range resp.Members {
		if m.Name == c.config.NodeName {
			log.Infof("[etcd] member: ID=%d Name=%q PeerURLs=%v IsLearner=%v",
				m.ID, m.Name, m.PeerURLs, m.IsLearner)
			return m.ID, nil
		}
	}

	return NotFoundID, nil
}

func (c *EtcdClient) PromoteMemberIfNeeded() error {
	memberId, err := c.CheckIfNodeIsLearner()
	if err != nil {
		return err
	}
	if memberId != NotFoundID {
		return c.MemberPromote(memberId)
	}
	return nil
}

func (c *EtcdClient) NewEtcdClient() (*clientv3.Client, error) {
	caCertPath := "/etc/kubernetes/pki/etcd/ca.crt"
	certPath := "/etc/kubernetes/pki/etcd/ca.crt"
	keyPath := "/etc/kubernetes/pki/etcd/ca.key"

	tlsConfig, err := c.buildTLSConfig(caCertPath, certPath, keyPath)
	if err != nil {
		return nil, err
	}

	cfg := clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	}
	return clientv3.New(cfg)
}

func (c *EtcdClient) buildTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caCertBytes, err := os.ReadFile(caFile)
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

// kubeadm function fork
func (c *EtcdClient) MemberPromote(learnerID uint64) error {
	log.Infof("[etcd] Promoting a learner as a voting member: %s", strconv.FormatUint(learnerID, 16))
	var err error
	var lastError error
	err = wait.ExponentialBackoff(etcdBackoff, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
		defer cancel()

		_, err = c.client.MemberPromote(ctx, learnerID)
		if err == nil {
			log.Infof("[etcd] The learner was promoted as a voting member: %s", strconv.FormatUint(learnerID, 16))
			return true, nil
		}
		log.Infof("[etcd] Promoting the learner %s failed: %v", strconv.FormatUint(learnerID, 16), err)
		lastError = err
		return false, nil
	})
	if err != nil {
		return lastError
	}
	return nil
}
