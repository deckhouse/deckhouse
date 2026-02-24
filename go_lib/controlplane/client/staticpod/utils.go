package staticpod

import (
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/etcdconfig"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmapi"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/client/kubeadmutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComponentPod returns a Pod object from the container, volume and annotations specifications
func ComponentPod(container v1.Container, volumes map[string]v1.Volume, annotations map[string]string) v1.Pod {
	// priority value for system-node-critical class
	priority := int32(2000001000)
	return v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: metav1.NamespaceSystem,
			// The component and tier labels are useful for quickly identifying the control plane Pods when doing a .List()
			// against Pods in the kube-system namespace. Can for example be used together with the WaitForPodsWithLabel function
			Labels:      map[string]string{"component": container.Name, "tier": constants.ControlPlaneTier},
			Annotations: annotations,
		},
		Spec: v1.PodSpec{
			Containers:        []v1.Container{container},
			Priority:          &priority,
			PriorityClassName: "system-node-critical",
			HostNetwork:       true,
			Volumes:           VolumeMapToSlice(volumes),
			SecurityContext: &v1.PodSecurityContext{
				SeccompProfile: &v1.SeccompProfile{
					Type: v1.SeccompProfileTypeRuntimeDefault,
				},
			},
		},
	}
}

// NewVolume creates a v1.Volume with a hostPath mount to the specified location
func NewVolume(name, path string, pathType *v1.HostPathType) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: path,
				Type: pathType,
			},
		},
	}
}

// NewVolumeMount creates a v1.VolumeMount to the specified location
func NewVolumeMount(name, path string, readOnly bool) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      name,
		MountPath: path,
		ReadOnly:  readOnly,
	}
}

// VolumeMapToSlice returns a slice of volumes from a map's values
func VolumeMapToSlice(volumes map[string]v1.Volume) []v1.Volume {
	v := make([]v1.Volume, 0, len(volumes))

	for _, vol := range volumes {
		v = append(v, vol)
	}

	sort.Slice(v, func(i, j int) bool {
		return strings.Compare(v[i].Name, v[j].Name) == -1
	})

	return v
}

// VolumeMountMapToSlice returns a slice of volumes from a map's values
func VolumeMountMapToSlice(volumeMounts map[string]v1.VolumeMount) []v1.VolumeMount {
	v := make([]v1.VolumeMount, 0, len(volumeMounts))

	for _, volMount := range volumeMounts {
		v = append(v, volMount)
	}

	sort.Slice(v, func(i, j int) bool {
		return strings.Compare(v[i].Name, v[j].Name) == -1
	})

	return v
}

// LivenessProbe creates a Probe object with a HTTPGet handler
func LivenessProbe(host, path, port string, scheme v1.URIScheme) *v1.Probe {
	// sets initialDelaySeconds same as periodSeconds to skip one period before running a check
	return createHTTPProbe(host, path, port, scheme, 10, 15, 8, 10)
}

// ReadinessProbe creates a Probe object with a HTTPGet handler
func ReadinessProbe(host, path, port string, scheme v1.URIScheme) *v1.Probe {
	// sets initialDelaySeconds as '0' because we don't want to delay user infrastructure checks
	// looking for "ready" status on kubeadm static Pods
	return createHTTPProbe(host, path, port, scheme, 0, 15, 3, 1)
}

// StartupProbe creates a Probe object with a HTTPGet handler
func StartupProbe(host, path, port string, scheme v1.URIScheme, timeoutForControlPlane *metav1.Duration) *v1.Probe {
	periodSeconds, timeoutForControlPlaneSeconds := int32(10), constants.ControlPlaneComponentHealthCheckTimeout.Seconds()
	if timeoutForControlPlane != nil {
		timeoutForControlPlaneSeconds = timeoutForControlPlane.Seconds()
	}
	// sets failureThreshold big enough to guarantee the full timeout can cover the worst case scenario for the control-plane to come alive
	// we ignore initialDelaySeconds in the calculation here for simplicity
	failureThreshold := int32(math.Ceil(timeoutForControlPlaneSeconds / float64(periodSeconds)))
	// sets initialDelaySeconds same as periodSeconds to skip one period before running a check
	return createHTTPProbe(host, path, port, scheme, periodSeconds, 15, failureThreshold, periodSeconds)
}

func createHTTPProbe(host, path, port string, scheme v1.URIScheme, initialDelaySeconds, timeoutSeconds, failureThreshold, periodSeconds int32) *v1.Probe {
	return &v1.Probe{
		ProbeHandler: v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Host:   host,
				Path:   path,
				Port:   intstr.FromString(port),
				Scheme: scheme,
			},
		},
		InitialDelaySeconds: initialDelaySeconds,
		TimeoutSeconds:      timeoutSeconds,
		FailureThreshold:    failureThreshold,
		PeriodSeconds:       periodSeconds,
	}
}

// GetEtcdProbeEndpoint takes a kubeadm Etcd configuration object and attempts to parse
// the first URL in the listen-metrics-urls argument, returning an etcd probe hostname,
// port and scheme
func GetEtcdProbeEndpoint(config *etcdconfig.EtcdConfig, isIPv6 bool) (string, int32, v1.URIScheme) {
	localhost := "127.0.0.1"
	if isIPv6 {
		localhost = "::1"
	}
	if config.LocalEtcd == nil || config.LocalEtcd.ExtraArgs == nil {
		return localhost, constants.EtcdMetricsPort, v1.URISchemeHTTP
	}
	if arg, idx := kubeadmapi.GetArgValue(config.LocalEtcd.ExtraArgs, "listen-metrics-urls", -1); idx > -1 {
		// Use the first url in the listen-metrics-urls if multiple URL's are specified.
		arg = strings.Split(arg, ",")[0]
		parsedURL, err := url.Parse(arg)
		if err != nil {
			return localhost, constants.EtcdMetricsPort, v1.URISchemeHTTP
		}
		// Parse scheme
		scheme := v1.URISchemeHTTP
		if parsedURL.Scheme == "https" {
			scheme = v1.URISchemeHTTPS
		}
		// Parse hostname
		hostname := parsedURL.Hostname()
		if len(hostname) == 0 {
			hostname = localhost
		}
		// Parse port
		port := constants.EtcdMetricsPort
		portStr := parsedURL.Port()
		if len(portStr) != 0 {
			p, err := kubeadmutil.ParsePort(portStr)
			if err == nil {
				port = p
			}
		}
		return hostname, int32(port), scheme
	}
	return localhost, constants.EtcdMetricsPort, v1.URISchemeHTTP
}
