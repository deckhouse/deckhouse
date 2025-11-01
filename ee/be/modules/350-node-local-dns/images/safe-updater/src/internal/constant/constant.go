/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package constant

import (
	"k8s.io/apimachinery/pkg/labels"
)

const (
	appLabel                   = "app"
	k8sAppLabel                = "k8s-app"
	PprofBindAddress           = ":4265"
	HealthProbeBindAddress     = ":4264"
	NodeLocalDNSDaemonSet      = "node-local-dns"
	NodeLocalDNSNamespace      = "kube-system"
	CiliumNamespace            = "d8-cni-cilium"
	ControllerName             = "node-local-dns-safe-updater"
	PodTemplateGenerationLabel = "pod-template-generation"
)

var (
	ControllerRevisionLabelSelector labels.Selector = labels.SelectorFromSet(map[string]string{appLabel: NodeLocalDNSDaemonSet, k8sAppLabel: NodeLocalDNSDaemonSet})
	NodeLocalDNSPodLabelSelector    labels.Selector = labels.SelectorFromSet(map[string]string{appLabel: NodeLocalDNSDaemonSet, k8sAppLabel: NodeLocalDNSDaemonSet})
)
