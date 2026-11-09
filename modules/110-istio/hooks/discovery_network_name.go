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

package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/adler32"
	"regexp"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	networkNameAnnotation = "istio.deckhouse.io/network-name"
	networkLabel          = "topology.istio.io/network"
	networkNameValuesPath = "istio.internal.networkName"

	networkNameMetricsGroup  = "network_name"
	networkNameInvalidMetric = "d8_istio_network_name_invalid"
)

// Label value charset without the length limit: old names may be longer.
// Keep in sync with openapi/values.yaml and istioNetworkLabel in templates/_helpers.tpl.
var networkNameRegexp = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(discoverNetworkName))

func discoverNetworkName(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	ns, err := getIstioNamespace(ctx, dc)
	if err != nil {
		return err
	}
	name, err := chooseNetworkName(input, ns)
	if err != nil {
		return err
	}
	input.Values.Set(networkNameValuesPath, name)

	input.MetricsCollector.Expire(networkNameMetricsGroup)
	if len(validation.IsValidLabelValue(name)) > 0 {
		input.MetricsCollector.Set(networkNameInvalidMetric, 1, map[string]string{"network_name": name}, metrics.WithGroup(networkNameMetricsGroup))
	}

	return nil
}

// getIstioNamespace returns nil if the namespace does not exist. It does not use a
// snapshot: an empty snapshot would look like a new installation.
func getIstioNamespace(ctx context.Context, dc dependency.Container) (*corev1.Namespace, error) {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("cannot get a Kubernetes client: %w", err)
	}
	ns, err := k8sClient.CoreV1().Namespaces().Get(ctx, istioNamespace, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot get the %s namespace to find the istio network name: %w", istioNamespace, err)
	}
	return ns, nil
}

// chooseNetworkName picks the name once, and Helm stores it in the networkNameAnnotation.
// It must not follow the cluster domain: parts of the mesh would switch at different times.
//
// The annotation wins, so editing it renames the network (sidecars only after a restart).
// Then the name in values wins, because Helm may not have stored it yet.
func chooseNetworkName(input *go_hook.HookInput, ns *corev1.Namespace) (string, error) {
	known := input.Values.Get(networkNameValuesPath).String()

	if name := storedNetworkName(ns); name != "" {
		if !networkNameRegexp.MatchString(name) {
			return "", fmt.Errorf("the %s annotation of the %s namespace holds %q, which is not a valid istio network name: it must match %s", networkNameAnnotation, istioNamespace, name, networkNameRegexp)
		}
		if known != "" && known != name {
			input.Logger.Warn("renaming the istio network after a change of the annotation", "annotation", networkNameAnnotation, "from", known, "to", name)
		}
		return name, nil
	}

	if known != "" {
		return known, nil
	}

	if ns == nil {
		name, err := newNetworkName(input)
		if err != nil {
			return "", err
		}
		input.Logger.Info("generating the istio network name of a new installation", "networkName", name)
		return name, nil
	}

	// An older release, or the annotation was lost. The label holds the name of the last
	// Helm run, if that release set it. It is never set for an invalid name.
	name := ns.GetLabels()[networkLabel]
	if name == "" {
		var err error
		if name, err = legacyNetworkName(input); err != nil {
			return "", err
		}
	}
	input.Logger.Info("adopting the istio network name of an existing installation", "networkName", name)
	return name, nil
}

func storedNetworkName(ns *corev1.Namespace) string {
	if ns == nil {
		return ""
	}
	return ns.GetAnnotations()[networkNameAnnotation]
}

// legacyNetworkName must stay byte-for-byte equal to what earlier releases rendered:
// `network-{{ include "istioClusterID" . }}`. Do not follow later changes of istioClusterID.
func legacyNetworkName(input *go_hook.HookInput) (string, error) {
	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()
	if clusterDomain == "" || clusterUUID == "" {
		return "", fmt.Errorf("cannot derive the istio network name: global.discovery.clusterDomain (%q) and global.discovery.clusterUUID (%q) must be set", clusterDomain, clusterUUID)
	}
	return "network-" + strings.ReplaceAll(clusterDomain, ".", "-") + "-" +
		strconv.FormatUint(uint64(adler32.Checksum([]byte(clusterUUID))), 10), nil
}

// newNetworkName is `network-` and 16 hex digits of sha256(clusterUUID): always a valid
// label value, independent of the cluster domain.
func newNetworkName(input *go_hook.HookInput) (string, error) {
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()
	if clusterUUID == "" {
		return "", fmt.Errorf("cannot generate the istio network name: global.discovery.clusterUUID is not set")
	}
	sum := sha256.Sum256([]byte(clusterUUID))
	return "network-" + hex.EncodeToString(sum[:])[:16], nil
}
