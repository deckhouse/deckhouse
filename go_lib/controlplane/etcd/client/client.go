/*
Copyright 2026 Flant JSC

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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
)

const etcdTimeout = 2 * time.Second

// Interface describes the etcd client surface used by controlplane code.
// It allows tests to stub promotion-related flows without a real clientv3 connection.
type Interface interface {
	Endpoints() []string
	Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error)
	WaitForClusterAvailable(retries int, retryInterval time.Duration) (bool, error)
	MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)
	MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error)
	Close() error
}

// Client wraps clientv3.Client so package-level methods can grow custom logic
// without exposing the raw etcd client as the primary API.
type Client struct {
	client        *clientv3.Client
	newEtcdClient func(endpoints []string) (Interface, error)
}

var _ Interface = (*Client)(nil)

func wrap(etcdClient *clientv3.Client) *Client {
	return &Client{client: etcdClient}
}

// Raw returns the underlying etcd client when direct access is required.
func (c *Client) Raw() *clientv3.Client {
	return c.client
}

func (c *Client) Endpoints() []string {
	return c.client.Endpoints()
}

func (c *Client) Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	return c.client.Status(ctx, endpoint)
}

func (c *Client) getMemberStatus(ctx context.Context, memberID uint64) (isLearner bool, started bool, err error) {
	//todo newClient?
	resp, err := c.client.MemberList(ctx)
	if err != nil {
		return false, false, err
	}

	var m *etcdserverpb.Member
	for _, member := range resp.Members {
		if member.ID == memberID {
			m = member
			break
		}
	}
	if m == nil {
		return false, false, fmt.Errorf("member %s not found", strconv.FormatUint(memberID, 16))
	}

	started = true
	// There is no field for "started".
	// If the member is not started, the Name and ClientURLs fields are set to their respective zero values.
	if len(m.Name) == 0 {
		started = false
	}

	return m.IsLearner, started, nil
}

func (c *Client) MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	return c.client.MemberAddAsLearner(ctx, peerAddrs)
}

// MemberPromote is the extension point for extra promotion logic around clientv3.
func (c *Client) MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error) {
	var (
		lastError     error
		learnerIDUint = strconv.FormatUint(id, 16)
	)
	logger.Info("etcd] Waiting for a learner to start", slog.String("learnerID", learnerIDUint))

	err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, constants.KubernetesAPICallTimeout,
		true, func(pollCtx context.Context) (bool, error) {
			isLearner, started, err := c.getMemberStatus(pollCtx, id)
			if err != nil {
				lastError = errors.WithMessagef(err, "failed to get member %s status", learnerIDUint)
				return false, nil
			}
			if !isLearner {
				logger.Info("member was already promoted.", slog.Any("memberID", learnerIDUint))
				return true, nil
			}
			if !started {
				logger.Info("member is not started yet. Waiting for it to be started.", slog.String("memberID", learnerIDUint))
				lastError = errors.Errorf("the etcd member %s is not started", learnerIDUint)
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		return nil, lastError
	}

	return c.client.MemberPromote(ctx, id)
}

func (c *Client) Close() error {
	return c.client.Close()
}

var logger = log.Default().Named("etcd-client")

// New creates an etcd client wrapper for the etcd endpoints present in etcd member list. In order to compose this information,
// it will first discover at least one etcd endpoint to connect to. Once created, the client synchronizes client's endpoints with
// the known endpoints from the etcd membership API, since it is the authoritative source of truth for the list of available members.
func New(client clientset.Interface, certificatesDir string) (Interface, error) {
	// Discover at least one etcd endpoint to connect to by inspecting the existing etcd pods

	// Get the list of etcd endpoints
	endpoints, err := getEtcdEndpoints(client)
	if err != nil {
		return nil, err
	}
	logger.Info("etcd endpoints read from pods", slog.String("endpoints", strings.Join(endpoints, ",")))

	cert, err := tls.LoadX509KeyPair(filepath.Join(certificatesDir, constants.EtcdHealthcheckClientCertName), filepath.Join(certificatesDir, constants.EtcdHealthcheckClientKeyName))
	if err != nil {
		return nil, err
	}
	caData, err := os.ReadFile(filepath.Join(certificatesDir, constants.EtcdCACertName))
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caData)

	rawClient, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		TLS: &tls.Config{
			RootCAs:      caPool,
			Certificates: []tls.Certificate{cert},
		},
	})
	if err != nil {
		return nil, err
	}

	return wrap(rawClient), nil
}

// getEtcdEndpoints returns the list of etcd endpoints.
func getEtcdEndpoints(client clientset.Interface) ([]string, error) {
	etcdEndpoints := []string{}
	var lastErr error
	// Let's tolerate some unexpected transient failures from the API server or load balancers. Also, if
	// static pods were not yet mirrored into the API server we want to wait for this propagation.
	err := wait.PollUntilContextTimeout(context.Background(), constants.KubernetesAPICallRetryInterval, constants.KubernetesAPICallTimeout, true,
		func(_ context.Context) (bool, error) {
			var overallEtcdPodCount int
			if etcdEndpoints, overallEtcdPodCount, lastErr = getRawEtcdEndpointsFromPodAnnotationWithoutRetry(client); lastErr != nil {
				return false, nil
			}
			if len(etcdEndpoints) == 0 || overallEtcdPodCount != len(etcdEndpoints) {
				//nolint:sloglint
				logger.Info("found etcd pods and endpoints; retrying", slog.Int("etcdPodCount", overallEtcdPodCount), slog.Any("endpoints", etcdEndpoints))
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		const message = "could not retrieve the list of etcd endpoints"
		if lastErr != nil {
			return []string{}, fmt.Errorf("%s: %w", message, lastErr)
		}
		return []string{}, fmt.Errorf("%s: %w", message, err)
	}
	return etcdEndpoints, nil
}

// getRawEtcdEndpointsFromPodAnnotationWithoutRetry returns the list of etcd endpoints as reported by etcd Pod annotations,
// along with the number of global etcd pods. This allows for callers to tell the difference between "no endpoints found",
// and "no endpoints found and pods were listed", so they can skip retrying.
func getRawEtcdEndpointsFromPodAnnotationWithoutRetry(client clientset.Interface) ([]string, int, error) {
	logger.Info("retrieving etcd endpoints from annotation in etcd Pods", slog.String("annotation", constants.EtcdAdvertiseClientUrlsAnnotationKey))
	podList, err := client.CoreV1().Pods(metav1.NamespaceSystem).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("component=%s,tier=%s", constants.Etcd, constants.ControlPlaneTier),
		},
	)
	if err != nil {
		return []string{}, 0, err
	}
	etcdEndpoints := []string{}
	for _, pod := range podList.Items {
		podIsReady := false
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				podIsReady = true
				break
			}
		}
		if !podIsReady {
			logger.Info("etcd pod is not ready", slog.String("pod", pod.ObjectMeta.Name))
		}
		etcdEndpoint, ok := pod.ObjectMeta.Annotations[constants.EtcdAdvertiseClientUrlsAnnotationKey]
		if !ok {
			logger.Info("etcd Pod is missing the annotation; cannot infer etcd advertise client URL using the Pod annotation", slog.String("pod", pod.ObjectMeta.Name), slog.String("annotation", constants.EtcdAdvertiseClientUrlsAnnotationKey))
			continue
		}
		etcdEndpoints = append(etcdEndpoints, etcdEndpoint)
	}
	return etcdEndpoints, len(podList.Items), nil
}

func (c *Client) getClusterStatus() (map[string]*clientv3.StatusResponse, error) {
	clusterStatus := make(map[string]*clientv3.StatusResponse)
	for _, ep := range c.Endpoints() {
		// Gets the member status
		var lastError error
		var resp *clientv3.StatusResponse
		err := wait.PollUntilContextTimeout(context.Background(), constants.EtcdAPICallRetryInterval, constants.KubernetesAPICallTimeout,
			true, func(_ context.Context) (bool, error) {
				cli, err := c.newEtcdClient(c.Endpoints())
				if err != nil {
					lastError = err
					return false, nil
				}
				defer func() { _ = cli.Close() }()

				ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
				resp, err = cli.Status(ctx, ep)
				cancel()
				if err == nil {
					return true, nil
				}
				logger.Error("Failed to get etcd status for", slog.Any("endpoint", ep), log.Err(err))
				lastError = err
				return false, nil
			})
		if err != nil {
			return nil, lastError
		}

		clusterStatus[ep] = resp
	}
	return clusterStatus, nil
}

func (c *Client) WaitForClusterAvailable(retries int, retryInterval time.Duration) (bool, error) {
	for i := 0; i < retries; i++ {
		if i > 0 {
			// nolint:sloglint
			logger.Info("Waiting until next retry", slog.Duration("retryInterval", retryInterval))
			time.Sleep(retryInterval)
		}
		endpoints := c.Endpoints()
		logger.Info("attempting to see if all cluster endpoints are available", slog.Any("endpoints", endpoints), slog.Int("attempt", i+1), slog.Int("retries", retries))
		_, err := c.getClusterStatus()
		if err != nil {
			switch err {
			case context.DeadlineExceeded:
				logger.Warn("Attempt timed out")
			default:
				logger.Warn("Attempt failed with error", slog.Any("error", err))
			}
			continue
		}
		return true, nil
	}
	return false, fmt.Errorf("timeout waiting for etcd cluster to be available")
}
