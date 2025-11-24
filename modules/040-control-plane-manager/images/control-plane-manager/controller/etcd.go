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
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	etcdTimeOut                          = 3 * time.Second
	apiCallRetryInterval                 = 2 * time.Second
	apiCallTimeout                       = 30 * time.Second
	EtcdAdvertiseClientUrlsAnnotationKey = "kubeadm.kubernetes.io/etcd.advertise-client-urls"
	EtcdComponent                        = "etcd"
	ControlPlaneTier                     = "control-plane"
	caCertPath                           = "/etc/kubernetes/pki/etcd/ca.crt"
	certPath                             = "/etc/kubernetes/pki/etcd/peer.crt"
	keyPath                              = "/etc/kubernetes/pki/etcd/peer.key"
)

var defaultETCDendpoints = []string{"https://127.0.0.1:2379"}

type Etcd struct {
	client *clientv3.Client
	wb     wait.Backoff
}

func EtcdJoinConverge() error {
	var etcdSubphase string

	if config.KubernetesVersion >= "1.33" {
		etcdSubphase = "etcd-join"
	} else {
		etcdSubphase = "etcd"
	}

	args := []string{"-v=5", "join", "phase", "control-plane-join", etcdSubphase, "--config", deckhousePath + "/kubeadm/config.yaml"}

	log.Info("run kubeadm",
		slog.String("phase", "etcd-join-converge"),
		slog.String("component", "etcd"),
		slog.String("kubernetes_version", config.KubernetesVersion),
		slog.Any("args", args),
	)

	cli := exec.Command(kubeadmPath, args...)
	out, err := cli.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Info("etcd join converge", slog.String("output", s))
	}
	return err
}

func (c *Etcd) findAllLearnerMembers() ([]uint64, error) {
	var (
		learnerIDs []uint64
		lastErr    error
		attempts   int
	)

	err := wait.ExponentialBackoff(c.wb, func() (bool, error) {
		attempts++

		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeOut)
		defer cancel()

		resp, err := c.client.MemberList(ctx)
		if err != nil {
			log.Infof("[d8][etcd] memberList failed on attempt %d: %v", attempts, err)
			lastErr = err
			return false, nil
		}

		var ids []uint64
		for _, m := range resp.Members {
			if m.IsLearner {
				log.Infof("[d8][etcd] Found learner member: ID=%d Name=%q PeerURLs=%v", m.ID, m.Name, m.PeerURLs)
				ids = append(ids, m.ID)
			}
		}
		learnerIDs = ids
		return true, nil
	})

	if err == wait.ErrWaitTimeout {
		log.Errorf("[d8][etcd] failed to list members after %d attempts: %v", attempts, lastErr)
		return nil, fmt.Errorf("[d8][etcd] memberList request failed: %v", lastErr)
	}
	return learnerIDs, err
}

func (c *Etcd) PromoteLearnersIfNeeded() error {
	learnerIDs, err := c.findAllLearnerMembers()
	if err != nil {
		return fmt.Errorf("[d8][etcd] failed to find learner members: %v", err)
	}

	if len(learnerIDs) == 0 {
		log.Info("[d8][etcd] No learner members found to promote")
		return nil
	}

	for _, memberID := range learnerIDs {
		err := c.MemberPromote(memberID)
		if err != nil {
			return fmt.Errorf("[d8][etcd] failed to promote member %s: %v", strconv.FormatUint(memberID, 16), err)
		}
	}
	return nil
}

func NewEtcd() (*Etcd, error) {
	var err error
	c := &Etcd{}
	if err != nil {
		return nil, err
	}
	c.client, err = c.newEtcdCli()
	if err != nil {
		return nil, err
	}
	c.wb = wait.Backoff{
		Steps:    10,
		Duration: 1 * time.Second,
		Factor:   1.5,
		Jitter:   0,
	}
	return c, nil
}

func (c *Etcd) newEtcdCli() (*clientv3.Client, error) {
	endpoints, err := c.getRawEtcdEndpointsFromPodAnnotation(apiCallRetryInterval, apiCallTimeout)

	// fallback
	if err != nil || len(endpoints) == 0 {
		log.Err(errors.Wrap(err, "[d8][etcd] cannot get etcd endpoints, fallback to default endpoint"))
		endpoints = defaultETCDendpoints
	}

	log.Infof("[d8][etcd] found etcd endpoints: %v", endpoints)

	tlsConfig, err := c.buildTLSConfig(caCertPath, certPath, keyPath)
	if err != nil {
		return nil, err
	}
	cfg := clientv3.Config{
		Endpoints:          endpoints,
		DialTimeout:        5 * time.Second,
		TLS:                tlsConfig,
		MaxUnaryRetries:    1,
		BackoffWaitBetween: 1 * time.Second,
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Etcd) buildTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	caCertBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("[d8][etcd] cannot read CA file %q: %v", caFile, err)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("[d8][etcd] cannot load client cert/key %q, %q: %v", certFile, keyFile, err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, fmt.Errorf("[d8][etcd] failed to parse CA certificate from %s", caFile)
	}

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}

func (c *Etcd) MemberPromote(learnerID uint64) error {
	log.Infof("[d8][etcd] promoting a learner as a voting member: %s", strconv.FormatUint(learnerID, 16))
	var lastError error
	attempts := 0
	err := wait.ExponentialBackoff(c.wb, func() (bool, error) {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeOut)
		defer cancel()

		_, err := c.client.MemberPromote(ctx, learnerID)
		if err == nil {
			log.Infof("[d8][etcd] learner was promoted as a voting member: %s after %d attempts", strconv.FormatUint(learnerID, 16), attempts)
			return true, nil
		}
		log.Infof("[d8][etcd] promoting the learner %s failed on attempt %d: %v", strconv.FormatUint(learnerID, 16), attempts, err)
		lastError = err
		return false, nil
	})

	if err == wait.ErrWaitTimeout {
		log.Errorf("[d8][etcd] failed to promote learner %s after %d attempts: %v", strconv.FormatUint(learnerID, 16), attempts, lastError)
		return lastError
	}
	return err
}

// getRawEtcdEndpointsFromPodAnnotation returns the list of endpoints as reported on etcd's pod annotations using the given backoff
// from kubeadm
func (c *Etcd) getRawEtcdEndpointsFromPodAnnotation(interval, timeout time.Duration) ([]string, error) {
	var etcdEndpoints []string
	var lastErr error

	err := wait.PollUntilContextTimeout(context.Background(), interval, timeout, true,
		func(_ context.Context) (bool, error) {
			var overallEtcdPodCount int
			if etcdEndpoints, overallEtcdPodCount, lastErr = c.getRawEtcdEndpointsFromPodAnnotationWithoutRetry(); lastErr != nil {
				return false, nil
			}
			if len(etcdEndpoints) == 0 || overallEtcdPodCount != len(etcdEndpoints) {
				log.Debugf("[d8][etcd] found a total of %d etcd pods and the following endpoints: %v; retrying",
					overallEtcdPodCount, etcdEndpoints)
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		const message = "[d8][etcd] could not retrieve the list of etcd endpoints"
		if lastErr != nil {
			return []string{}, errors.Wrap(lastErr, message)
		}
		return []string{}, errors.Wrap(err, message)
	}
	return etcdEndpoints, nil
}

// getRawEtcdEndpointsFromPodAnnotationWithoutRetry returns the list of etcd endpoints as reported by etcd Pod annotations,
// along with the number of global etcd pods. This allows for callers to tell the difference between "no endpoints found",
// and "no endpoints found and pods were listed", so they can skip retrying.
// from kubeadm
func (c *Etcd) getRawEtcdEndpointsFromPodAnnotationWithoutRetry() ([]string, int, error) {
	log.Debugf("[d8][etcd] retrieving etcd endpoints from %q annotation in etcd Pods", EtcdAdvertiseClientUrlsAnnotationKey)
	podList, err := config.K8sClient.CoreV1().Pods(metav1.NamespaceSystem).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s,tier=%s", EtcdComponent, ControlPlaneTier),
		},
	)
	if err != nil {
		return []string{}, 0, err
	}

	etcdEndpoints := make([]string, 0)
	for _, pod := range podList.Items {
		podIsReady := false
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				podIsReady = true
				break
			}
		}
		if !podIsReady {
			log.Debugf("[d8][etcd] etcd pod %q is not ready", pod.ObjectMeta.Name)
		}
		etcdEndpoint, ok := pod.ObjectMeta.Annotations[EtcdAdvertiseClientUrlsAnnotationKey]
		if !ok {
			log.Debugf("[d8][etcd] etcd Pod %q is missing the %q annotation; cannot infer etcd advertise client URL using the Pod annotation", pod.ObjectMeta.Name, EtcdAdvertiseClientUrlsAnnotationKey)
			continue
		}
		etcdEndpoints = append(etcdEndpoints, etcdEndpoint)
	}
	return etcdEndpoints, len(podList.Items), nil
}
