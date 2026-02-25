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

// // LoadInitConfigurationFromFile loads a supported versioned InitConfiguration from a file, converts it into internal config, defaults it and verifies it.
// func LoadInitConfigurationFromFile(cfgPath string, opts kubeadmutil.LoadOrDefaultConfigurationOptions) (*kubeadmapi.InitConfiguration, error) {
// 	klog.V(1).Infof("loading configuration from %q", cfgPath)

// 	b, err := os.ReadFile(cfgPath)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "unable to read config from %q ", cfgPath)
// 	}

// 	return BytesToInitConfiguration(b, opts.SkipCRIDetect)
// }

// // LoadOrDefaultInitConfiguration takes a path to a config file and a versioned configuration that can serve as the default config
// // If cfgPath is specified, the versioned configs will always get overridden with the one in the file (specified by cfgPath).
// // The external, versioned configuration is defaulted and converted to the internal type.
// // Right thereafter, the configuration is defaulted again with dynamic values (like IP addresses of a machine, etc)
// // Lastly, the internal config is validated and returned.
// func LoadOrDefaultInitConfiguration(cfgPath string, versionedInitCfg *kubeadmapiv1.InitConfiguration, versionedClusterCfg *kubeadmapiv1.ClusterConfiguration, opts LoadOrDefaultConfigurationOptions) (*kubeadmapi.InitConfiguration, error) {
// 	var (
// 		config *kubeadmapi.InitConfiguration
// 		err    error
// 	)
// 	if cfgPath != "" {
// 		// Loads configuration from config file, if provided
// 		config, err = LoadInitConfigurationFromFile(cfgPath, opts)
// 	} else {
// 		config, err = DefaultedInitConfiguration(versionedInitCfg, versionedClusterCfg, opts)
// 	}
// 	if err == nil {
// 		prepareStaticVariables(config)
// 	}
// 	return config, err
// }

// // BytesToInitConfiguration converts a byte slice to an internal, defaulted and validated InitConfiguration object.
// // The map may contain many different YAML/JSON documents. These documents are parsed one-by-one
// // and well-known ComponentConfig GroupVersionKinds are stored inside of the internal InitConfiguration struct.
// // The resulting InitConfiguration is then dynamically defaulted and validated prior to return.
// func BytesToInitConfiguration(b []byte, skipCRIDetect bool) (*kubeadmapi.InitConfiguration, error) {
// 	// Split the YAML/JSON documents in the file into a DocumentMap
// 	gvkmap, err := kubeadmutil.SplitConfigDocuments(b)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return documentMapToInitConfiguration(gvkmap, false, false, false, skipCRIDetect)
// }

// // documentMapToInitConfiguration converts a map of GVKs and YAML/JSON documents to defaulted and validated configuration object.
// func documentMapToInitConfiguration(gvkmap kubeadmapi.DocumentMap, allowDeprecated, allowExperimental, strictErrors, skipCRIDetect bool) (*kubeadmapi.InitConfiguration, error) {
// 	var initcfg *kubeadmapi.InitConfiguration
// 	var clustercfg *kubeadmapi.ClusterConfiguration

// 	// Sort the GVKs deterministically by GVK string.
// 	// This allows ClusterConfiguration to be decoded first.
// 	gvks := make([]schema.GroupVersionKind, 0, len(gvkmap))
// 	for gvk := range gvkmap {
// 		gvks = append(gvks, gvk)
// 	}
// 	sort.Slice(gvks, func(i, j int) bool {
// 		return gvks[i].String() < gvks[j].String()
// 	})

// 	for _, gvk := range gvks {
// 		fileContent := gvkmap[gvk]

// 		// first, check if this GVK is supported and possibly not deprecated
// 		if err := validateSupportedVersion(gvk, allowDeprecated, allowExperimental); err != nil {
// 			return nil, err
// 		}

// 		// verify the validity of the JSON/YAML
// 		if err := strict.VerifyUnmarshalStrict([]*runtime.Scheme{kubeadmscheme.Scheme, componentconfigs.Scheme}, gvk, fileContent); err != nil {
// 			if !strictErrors {
// 				klog.Warning(err.Error())
// 			} else {
// 				return nil, err
// 			}
// 		}

// 		if kubeadmutil.GroupVersionKindsHasInitConfiguration(gvk) {
// 			// Set initcfg to an empty struct value the deserializer will populate
// 			initcfg = &kubeadmapi.InitConfiguration{}
// 			// Decode the bytes into the internal struct. Under the hood, the bytes will be unmarshalled into the
// 			// right external version, defaulted, and converted into the internal version.
// 			if err := runtime.DecodeInto(kubeadmscheme.Codecs.UniversalDecoder(), fileContent, initcfg); err != nil {
// 				return nil, err
// 			}
// 			continue
// 		}
// 		if kubeadmutil.GroupVersionKindsHasClusterConfiguration(gvk) {
// 			// Set clustercfg to an empty struct value the deserializer will populate
// 			clustercfg = &kubeadmapi.ClusterConfiguration{}
// 			// Decode the bytes into the internal struct. Under the hood, the bytes will be unmarshalled into the
// 			// right external version, defaulted, and converted into the internal version.
// 			if err := runtime.DecodeInto(kubeadmscheme.Codecs.UniversalDecoder(), fileContent, clustercfg); err != nil {
// 				return nil, err
// 			}
// 			continue
// 		}

// 		// If the group is neither a kubeadm core type or of a supported component config group, we dump a warning about it being ignored
// 		if !componentconfigs.Scheme.IsGroupRegistered(gvk.Group) {
// 			klog.Warningf("[config] WARNING: Ignored configuration document with GroupVersionKind %v\n", gvk)
// 		}
// 	}

// 	// Enforce that InitConfiguration and/or ClusterConfiguration has to exist among the configuration documents
// 	if initcfg == nil && clustercfg == nil {
// 		return nil, errors.New("no InitConfiguration or ClusterConfiguration kind was found in the configuration file")
// 	}

// 	// If InitConfiguration wasn't given, default it by creating an external struct instance, default it and convert into the internal type
// 	if initcfg == nil {
// 		extinitcfg := &kubeadmapiv1.InitConfiguration{}
// 		kubeadmscheme.Scheme.Default(extinitcfg)
// 		// Set initcfg to an empty struct value the deserializer will populate
// 		initcfg = &kubeadmapi.InitConfiguration{}
// 		if err := kubeadmscheme.Scheme.Convert(extinitcfg, initcfg, nil); err != nil {
// 			return nil, err
// 		}
// 	}
// 	// If ClusterConfiguration was given, populate it in the InitConfiguration struct
// 	if clustercfg != nil {
// 		initcfg.ClusterConfiguration = *clustercfg

// 		// TODO: Workaround for missing v1beta3 ClusterConfiguration timeout conversion. Remove this conversion once the v1beta3 is removed
// 		if clustercfg.APIServer.TimeoutForControlPlane.Duration != 0 && clustercfg.APIServer.TimeoutForControlPlane.Duration != kubeadmconstants.ControlPlaneComponentHealthCheckTimeout {
// 			initcfg.Timeouts.ControlPlaneComponentHealthCheck.Duration = clustercfg.APIServer.TimeoutForControlPlane.Duration
// 		}
// 	} else {
// 		// Populate the internal InitConfiguration.ClusterConfiguration with defaults
// 		extclustercfg := &kubeadmapiv1.ClusterConfiguration{}
// 		kubeadmscheme.Scheme.Default(extclustercfg)
// 		if err := kubeadmscheme.Scheme.Convert(extclustercfg, &initcfg.ClusterConfiguration, nil); err != nil {
// 			return nil, err
// 		}
// 	}

// 	// Load any component configs
// 	if err := componentconfigs.FetchFromDocumentMap(&initcfg.ClusterConfiguration, gvkmap); err != nil {
// 		return nil, err
// 	}

// 	// Applies dynamic defaults to settings not provided with flags
// 	if err := SetInitDynamicDefaults(initcfg, skipCRIDetect); err != nil {
// 		return nil, err
// 	}

// 	// Validates cfg (flags/configs + defaults + dynamic defaults)
// 	if err := validation.ValidateInitConfiguration(initcfg).ToAggregate(); err != nil {
// 		return nil, err
// 	}

// 	return initcfg, nil
// }
