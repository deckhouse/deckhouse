/*
Copyright 2025 Flant JSC

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
	etcdPromoteTimeout        = 3 * time.Second
	NotFoundID         uint64 = 0
	etcdEndpoint              = "https://127.0.0.1:2379"
)

var etcdBackoff = wait.Backoff{
	Steps:    10,
	Duration: 1 * time.Second,
	Factor:   1.5,
	Jitter:   0.1,
}

type EtcdClient struct {
	client *clientv3.Client
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

func (c *EtcdClient) CheckIfNodeIsLearner() (uint64, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.MemberList(ctx)
	if err != nil {
		return NotFoundID, false, fmt.Errorf("MemberList request failed: %v", err)
	}

	for _, m := range resp.Members {
		if m.Name == config.NodeName {
			log.Infof("[etcd] member: ID=%d Name=%q PeerURLs=%v IsLearner=%v",
				m.ID, m.Name, m.PeerURLs, m.IsLearner)
			return m.ID, m.IsLearner, nil
		}
	}
	return NotFoundID, false, nil
}

func (c *EtcdClient) PromoteMemberIfNeeded() error {
	memberId, isLearner, err := c.CheckIfNodeIsLearner()
	if err != nil {
		return err
	}
	if isLearner {
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
	// etcd client with backoff
	cfg := clientv3.Config{
		Endpoints:          []string{etcdEndpoint},
		DialTimeout:        5 * time.Second,
		TLS:                tlsConfig,
		MaxUnaryRetries:    10,
		BackoffWaitBetween: 1 * time.Second,
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
	attempts := 0

	err = wait.ExponentialBackoff(etcdBackoff, func() (bool, error) {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), etcdPromoteTimeout)
		defer cancel()

		_, err = c.client.MemberPromote(ctx, learnerID)
		if err == nil {
			log.Infof("[etcd] The learner was promoted as a voting member: %s after %d attempts", strconv.FormatUint(learnerID, 16), attempts)
			return true, nil
		}
		log.Infof("[etcd] Promoting the learner %s failed on attempt %d: %v", strconv.FormatUint(learnerID, 16), attempts, err)
		lastError = err
		return false, nil
	})
	if err != nil {
		log.Errorf("[etcd] Failed to promote learner %s after %d attempts: %v", strconv.FormatUint(learnerID, 16), attempts, lastError)
		return lastError
	}
	return nil
}
