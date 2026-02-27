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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/etcd"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/etcdconfig"
	kubeadmapi "github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	"github.com/deckhouse/deckhouse/pkg/log"
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

type EtcdPerformanceParams struct {
	HeartbeatInterval int
	ElectionTimeout   int
}

type EtcdMember struct {
	NodeName  string
	IsLearner bool
	ID        uint64
}

func GetEtcdPerformanceParams() EtcdPerformanceParams {
	defaultParams := EtcdPerformanceParams{
		HeartbeatInterval: 100,
		ElectionTimeout:   1000,
	}

	etcdArbiterParams := EtcdPerformanceParams{
		HeartbeatInterval: 500,
		ElectionTimeout:   5000,
	}

	if config.EtcdArbiter {
		log.Info("using increased etcd timeouts for EtcdArbiter mode", slog.Int("heartbeat_interval_ms", etcdArbiterParams.HeartbeatInterval), slog.Int("election_timeout_ms", etcdArbiterParams.ElectionTimeout))
		return etcdArbiterParams
	}

	return defaultParams
}

// We can use this patch to tune etcd performance depending on the disk performance in future
func GenerateEtcdPerformancePatch(params EtcdPerformanceParams) error {
	const patchTemplate = `apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
    - name: etcd
      env:
        - name: ETCD_HEARTBEAT_INTERVAL
          value: "%d"
        - name: ETCD_ELECTION_TIMEOUT
          value: "%d"`

	patchFile := filepath.Join(deckhousePath, "kubeadm", "patches", "etcd800performance.yaml")
	content := fmt.Sprintf(patchTemplate, params.HeartbeatInterval, params.ElectionTimeout)

	log.Info("generating etcd performance patch", slog.String("file", patchFile), slog.Int("heartbeat_interval_ms", params.HeartbeatInterval), slog.Int("election_timeout_ms", params.ElectionTimeout))

	return os.WriteFile(patchFile, []byte(content), 0o600)
}

func EtcdJoinConverge() error {
	var args []string
	v, err := semver.NewVersion(config.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("version not being parsable: %s", err.Error())
	}
	c, err := semver.NewConstraint(">= 1.33")
	if err != nil {
		return fmt.Errorf("constraint not being parsable: %s", err.Error())
	}
	if c.Check(v) { // >= 1.33
		args = []string{"-v=5", "join", "phase", "etcd-join", "--config", deckhousePath + "/kubeadm/config.yaml"}
	} else {
		args = []string{"-v=5", "join", "phase", "control-plane-join", "etcd", "--config", deckhousePath + "/kubeadm/config.yaml"}
	}

	log.Info("run kubeadm",
		slog.String("phase", "etcd-join-converge"),
		slog.String("component", "etcd"),
		slog.String("kubernetes_version", config.KubernetesVersion),
		slog.Any("args", args),
	)

	etcdManifest := `apiVersion: v1
kind: Pod
metadata:
  annotations:
    control-plane-manager.deckhouse.io/checksum: f46ba03abdacb1acb00b769ecbf07a8a61988b5c4812f6141b70b70f3a42c4e3
    kubeadm.kubernetes.io/etcd.advertise-client-urls: https://10.12.1.32:2379
  creationTimestamp: null
  labels:
    component: etcd
    tier: control-plane
  name: etcd
  namespace: kube-system
spec:
  containers:
  - command:
    - etcd
    - --advertise-client-urls=https://10.12.1.32:2379
    - --cert-file=/etc/kubernetes/pki/etcd/server.crt
    - --client-cert-auth=true
    - --data-dir=/var/lib/etcd
    - --experimental-initial-corrupt-check=true
    - --experimental-watch-progress-notify-interval=5s
    - --initial-advertise-peer-urls=https://10.12.1.32:2380
    - --initial-cluster=dkp-borovets-master-0=https://10.12.1.32:2380
    - --initial-cluster-state=existing
    - --key-file=/etc/kubernetes/pki/etcd/server.key
    - --listen-client-urls=https://127.0.0.1:2379,https://10.12.1.32:2379
    - --listen-metrics-urls=http://127.0.0.1:2381
    - --listen-peer-urls=https://10.12.1.32:2380
    - --metrics=extensive
    - --name=dkp-borovets-master-0
    - --peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt
    - --peer-client-cert-auth=true
    - --peer-key-file=/etc/kubernetes/pki/etcd/peer.key
    - --peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    - --quota-backend-bytes=2147483648
    - --snapshot-count=10000
    - --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    image: dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:b4bcee9498f54dcf0ee4377e5dcf9f98788517f7de1ccf10eeaba61a9a5f7337
    imagePullPolicy: IfNotPresent
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /livez
        port: 2381
        scheme: HTTP
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    name: etcd
    readinessProbe:
      failureThreshold: 3
      httpGet:
        host: 127.0.0.1
        path: /health
        port: 2381
        scheme: HTTP
      periodSeconds: 1
      timeoutSeconds: 15
    resources:
      requests:
        cpu: 518m
        memory: "1127428915"
    securityContext:
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
      runAsGroup: 0
      runAsNonRoot: false
      runAsUser: 0
      seccompProfile:
        type: RuntimeDefault
    startupProbe:
      failureThreshold: 24
      httpGet:
        host: 127.0.0.1
        path: /readyz?exclude=non_learner
        port: 2381
        scheme: HTTP
      initialDelaySeconds: 10
      periodSeconds: 10
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /var/lib/etcd
      name: etcd-data
    - mountPath: /etc/kubernetes/pki/etcd
      name: etcd-certs
  dnsPolicy: ClusterFirstWithHostNet
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
  - hostPath:
      path: /etc/kubernetes/pki/etcd
      type: DirectoryOrCreate
    name: etcd-certs
  - hostPath:
      path: /var/lib/etcd
      type: DirectoryOrCreate
    name: etcd-data
status: {}`

	advertiseAddress := "10.12.1.32"
	nodeName := "dkp-borovets-master-0"

	config := &etcdconfig.EtcdConfig{}

	// var kubeClient clientset.Interface

	if err := etcd.JoinCluster([]byte(etcdManifest) /*kubeClient,*/, nil, config, &kubeadmapi.APIEndpoint{AdvertiseAddress: advertiseAddress}, nodeName, false); err != nil {
		log.Error("failed to test etcd library JoinCluster", log.Err(err))
	}

	cli := exec.Command(kubeadmPath, args...)
	out, err := cli.CombinedOutput()
	for _, s := range strings.Split(string(out), "\n") {
		log.Info("etcd join converge", slog.String("output", s))
	}
	return err
}

func (c *Etcd) findAllMembers() ([]EtcdMember, error) {
	var (
		members  []EtcdMember
		lastErr  error
		attempts int
	)
	err := wait.ExponentialBackoff(c.wb, func() (bool, error) {
		attempts++

		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeOut)
		defer cancel()

		resp, err := c.client.MemberList(ctx)
		if err != nil {
			log.Info("[d8][etcd] memberList failed", slog.Int("attempt", attempts), slog.Any("error", err))
			lastErr = err
			return false, nil
		}
		for _, m := range resp.Members {
			members = append(members, EtcdMember{
				NodeName:  m.Name,
				IsLearner: m.IsLearner,
				ID:        m.ID,
			})
		}
		return true, nil
	})
	if wait.Interrupted(err) {
		log.Error("[d8][etcd] failed to list members after a number of attempts", slog.Int("attempt", attempts), slog.Any("error", lastErr))
		return nil, fmt.Errorf("[d8][etcd] memberList request failed: %v", lastErr)
	}
	return members, nil
}

func (c *Etcd) checkMemberExists(nodeName string) (bool, error) {
	members, err := c.findAllMembers()
	if err != nil {
		return false, fmt.Errorf("[d8][etcd] failed to find all members: %v", err)
	}
	for _, member := range members {
		if member.NodeName == nodeName {
			return true, nil
		}
	}
	return false, nil
}

func (c *Etcd) findAllLearnerMembers() ([]uint64, error) {
	var learnerIDs []uint64
	members, err := c.findAllMembers()
	if err != nil {
		return nil, fmt.Errorf("[d8][etcd] failed to find all members: %v", err)
	}
	for _, member := range members {
		if member.IsLearner {
			learnerIDs = append(learnerIDs, member.ID)
		}
	}

	return learnerIDs, nil
}

func (c *Etcd) promoteLearnersIfNeeded() error {
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

	log.Info("[d8][etcd] found etcd endpoints", slog.Any("endpoints", endpoints))

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
	log.Info("[d8][etcd] promoting a learner as a voting member", slog.String("learner_id", strconv.FormatUint(learnerID, 16)))
	var lastError error
	attempts := 0
	err := wait.ExponentialBackoff(c.wb, func() (bool, error) {
		attempts++
		ctx, cancel := context.WithTimeout(context.Background(), etcdTimeOut)
		defer cancel()

		_, err := c.client.MemberPromote(ctx, learnerID)
		if err == nil {
			log.Info("[d8][etcd] learner was promoted as a voting member", slog.String("learner_id", strconv.FormatUint(learnerID, 16)), slog.Int("attempt", attempts))
			return true, nil
		}
		log.Info("[d8][etcd] promoting the learner failed", slog.String("learner_id", strconv.FormatUint(learnerID, 16)), slog.Int("attempt", attempts), slog.Any("error", err))
		lastError = err
		return false, nil
	})

	if wait.Interrupted(err) {
		log.Error("[d8][etcd] failed to promote learner", slog.String("learner_id", strconv.FormatUint(learnerID, 16)), slog.Int("attempt", attempts), slog.Any("error", lastError))
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
				log.Debug("[d8][etcd] found a number of etcd pods and etcd endpoints; retrying", slog.Int("etcd_pod_count", overallEtcdPodCount), slog.Any("etcd_endpoints", etcdEndpoints))
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
	log.Debug("[d8][etcd] retrieving etcd endpoints from the annotation in etcd Pods", slog.String("annotation_key", EtcdAdvertiseClientUrlsAnnotationKey))
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
			log.Debug("[d8][etcd] etcd pod is not ready", slog.String("pod", pod.ObjectMeta.Name))
		}
		etcdEndpoint, ok := pod.ObjectMeta.Annotations[EtcdAdvertiseClientUrlsAnnotationKey]
		if !ok {
			log.Debug("[d8][etcd] etcd Pod is missing the annotation; cannot infer etcd advertise client URL using the Pod annotation", slog.String("pod", pod.ObjectMeta.Name), slog.String("annotation_key", EtcdAdvertiseClientUrlsAnnotationKey))
			continue
		}
		etcdEndpoints = append(etcdEndpoints, etcdEndpoint)
	}
	return etcdEndpoints, len(podList.Items), nil
}
