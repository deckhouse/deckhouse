package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/constants"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

// NewFromCluster creates an etcd client for the etcd endpoints present in etcd member list. In order to compose this information,
// it will first discover at least one etcd endpoint to connect to. Once created, the client synchronizes client's endpoints with
// the known endpoints from the etcd membership API, since it is the authoritative source of truth for the list of available members.
func NewFromCluster(client clientset.Interface, certificatesDir string) (*clientv3.Client, error) {
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

	etcdClient, err := clientv3.New(clientv3.Config{
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

	// was this block
	// synchronizes client's endpoints with the known endpoints from the etcd membership.
	// err = etcdClient.Sync()

	return etcdClient, nil
}

// WaitForClusterAvailable returns true if all endpoints in the cluster are available after retry attempts, an error is returned otherwise
func WaitForClusterAvailable(c *clientv3.Client, retries int, retryInterval time.Duration) (bool, error) {
	for i := 0; i < retries; i++ {
		if i > 0 {
			logger.Info("[etcd] Waiting until next retry", slog.Duration("retryInterval", retryInterval))
			time.Sleep(retryInterval)
		}
		logger.Info("[etcd] attempting to see if all cluster endpoints are available", slog.Any("endpoints", c.Endpoints), slog.Int("attempt", i+1), slog.Int("retries", retries))
		_, err := c.Status(context.Background(), c.Endpoints()[0])
		if err != nil {
			switch err {
			case context.DeadlineExceeded:
				logger.Info("[etcd] Attempt timed out")
			default:
				logger.Info("[etcd] Attempt failed with error", slog.Any("error", err))
			}
			continue
		}
		return true, nil
	}
	return false, errors.New("timeout waiting for etcd cluster to be available")
}

// getEtcdEndpoints returns the list of etcd endpoints.
func getEtcdEndpoints(client clientset.Interface) ([]string, error) {
	return getEtcdEndpointsWithRetry(client,
		constants.KubernetesAPICallRetryInterval, KubernetesAPICallTimeout)
}

func getEtcdEndpointsWithRetry(client clientset.Interface, interval, timeout time.Duration) ([]string, error) {
	return getRawEtcdEndpointsFromPodAnnotation(client, interval, timeout)
}

// getRawEtcdEndpointsFromPodAnnotation returns the list of endpoints as reported on etcd's pod annotations using the given backoff
func getRawEtcdEndpointsFromPodAnnotation(client clientset.Interface, interval, timeout time.Duration) ([]string, error) {
	etcdEndpoints := []string{}
	var lastErr error
	// Let's tolerate some unexpected transient failures from the API server or load balancers. Also, if
	// static pods were not yet mirrored into the API server we want to wait for this propagation.
	err := wait.PollUntilContextTimeout(context.Background(), interval, timeout, true,
		func(_ context.Context) (bool, error) {
			var overallEtcdPodCount int
			if etcdEndpoints, overallEtcdPodCount, lastErr = getRawEtcdEndpointsFromPodAnnotationWithoutRetry(client); lastErr != nil {
				return false, nil
			}
			if len(etcdEndpoints) == 0 || overallEtcdPodCount != len(etcdEndpoints) {
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

// GetPeerURL creates an HTTPS URL that uses the configured advertise
// address and peer port for the API controller
func GetPeerURL(ip string) string {
	return "https://" + net.JoinHostPort(ip, strconv.Itoa(constants.EtcdListenPeerPort))
}

// GetStaticPodFilepath returns the location on the disk where the Static Pod should be present
func GetStaticPodFilepath(componentName, manifestsDir string) string {
	return filepath.Join(manifestsDir, componentName+".yaml")
}
