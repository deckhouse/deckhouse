package etcd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	constants "github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	errors "github.com/deckhouse/deckhouse/go_lib/controlplane/client/errors"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type Member struct {
	Name    string
	PeerURL string
}

// getEtcdEndpoints returns the list of etcd endpoints.
func getEtcdEndpoints(client clientset.Interface) ([]string, error) {
	return getEtcdEndpointsWithRetry(client,
		constants.KubernetesAPICallRetryInterval, kubeadmapi.GetActiveTimeouts().KubernetesAPICall.Duration)
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
				klog.V(4).Infof("found a total of %d etcd pods and the following endpoints: %v; retrying",
					overallEtcdPodCount, etcdEndpoints)
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		const message = "could not retrieve the list of etcd endpoints"
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
func getRawEtcdEndpointsFromPodAnnotationWithoutRetry(client clientset.Interface) ([]string, int, error) {
	klog.V(3).Infof("retrieving etcd endpoints from %q annotation in etcd Pods", constants.EtcdAdvertiseClientUrlsAnnotationKey)
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
			klog.V(3).Infof("etcd pod %q is not ready", pod.ObjectMeta.Name)
		}
		etcdEndpoint, ok := pod.ObjectMeta.Annotations[constants.EtcdAdvertiseClientUrlsAnnotationKey]
		if !ok {
			klog.V(3).Infof("etcd Pod %q is missing the %q annotation; cannot infer etcd advertise client URL using the Pod annotation", pod.ObjectMeta.Name, constants.EtcdAdvertiseClientUrlsAnnotationKey)
			continue
		}
		etcdEndpoints = append(etcdEndpoints, etcdEndpoint)
	}
	return etcdEndpoints, len(podList.Items), nil
}

// GetClientURL creates an HTTPS URL that uses the configured advertise
// address and client port for the API controller
func GetClientURL(localEndpoint *kubeadmapi.APIEndpoint) string {
	return "https://" + net.JoinHostPort(localEndpoint.AdvertiseAddress, strconv.Itoa(constants.EtcdListenClientPort))
}

// GetPeerURL creates an HTTPS URL that uses the configured advertise
// address and peer port for the API controller
func GetPeerURL(localEndpoint *kubeadmapi.APIEndpoint) string {
	return "https://" + net.JoinHostPort(localEndpoint.AdvertiseAddress, strconv.Itoa(constants.EtcdListenPeerPort))
}

// GetClientURLByIP creates an HTTPS URL based on an IP address
// and the client listening port.
func GetClientURLByIP(ip string) string {
	return "https://" + net.JoinHostPort(ip, strconv.Itoa(constants.EtcdListenClientPort))
}
