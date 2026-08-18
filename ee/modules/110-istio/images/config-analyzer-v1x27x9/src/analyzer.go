/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

//go:build deckhouse_external

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"istio.io/istio/pkg/config/analysis/analyzers"
	"istio.io/istio/pkg/config/analysis/diag"
	"istio.io/istio/pkg/config/analysis/local"
	"istio.io/istio/pkg/config/resource"
	"istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/log"
)

var outputThreshold = diag.Info

func runAnalysis(ctx context.Context, istioNamespace, revision string, allNamespaces bool) ([]diag.Message, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	client, err := kube.NewCLIClient(clientConfig, kube.WithRevision(revision))
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	selectedNamespace := metav1.NamespaceDefault
	if allNamespaces {
		selectedNamespace = ""
	}

	sa := local.NewIstiodAnalyzer(
		analyzers.AllCombined(),
		resource.Namespace(selectedNamespace),
		resource.Namespace(istioNamespace),
		nil,
	)

	k := kube.EnableCrdWatcher(client)
	sa.AddRunningKubeSourceWithRevision(k, revision, false)

	cancel := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			close(cancel)
		case <-cancel:
		}
	}()

	result, err := sa.Analyze(cancel)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	messages := make([]diag.Message, 0, len(result.Messages))
	for _, message := range result.Messages {
		if shouldReportMessage(message) {
			messages = append(messages, message)
		}
	}

	log.Infof("analysis completed: revision=%s messages=%d", revision, len(messages))
	return messages, nil
}

const (
	deckhouseSystemNamespacePrefix = "d8-"
	sidecarInjectLabel             = "sidecar.istio.io/inject"
)

// mutedSystemNamespaces are Kubernetes/Deckhouse system namespaces where
// IST0102 / IST0118 findings are noise (not mesh application workloads).
var mutedSystemNamespaces = []string{
	"kube-system",
	"kube-public",
	"kube-node-lease",
}

// mutedCodesForDeckhouseSystem are Info-level findings that are noise for
// Deckhouse system namespaces (d8-*), including d8-ingress-nginx / d8-ingress-istio,
// and other system namespaces listed in mutedSystemNamespaces:
// those namespaces are not meant for namespace-wide sidecar injection, and Service
// ports there often do not follow Istio naming because the Services are not mesh
// workloads (including operator-created ones like prometheus-operated / vpa-webhook).
//
// Exception: resources explicitly opted into sidecar injection via
// label/annotation sidecar.istio.io/inject=true (e.g. IngressNginxController
// with enableIstioSidecar) keep all findings — they are real mesh workloads.
var mutedCodesForDeckhouseSystem = map[string]struct{}{
	"IST0102": {}, // NamespaceNotInjected
	"IST0118": {}, // PortNameIsNotUnderNamingConvention
}

func shouldReportMessage(message diag.Message) bool {
	if !message.Type.Level().IsWorseThanOrEqualTo(outputThreshold) {
		return false
	}
	if _, muted := mutedCodesForDeckhouseSystem[message.Type.Code()]; muted &&
		isMutedSystemNamespaceResource(message) &&
		!hasIstioSidecarInject(message) {
		return false
	}
	return true
}

func isMutedSystemNamespaceResource(message diag.Message) bool {
	if message.Resource == nil {
		return false
	}
	name := string(message.Resource.Metadata.FullName.Name)
	ns := string(message.Resource.Metadata.FullName.Namespace)
	// Namespace objects are cluster-scoped: name is the namespace itself.
	return isMutedSystemNamespace(ns) || isMutedSystemNamespace(name)
}

func hasIstioSidecarInject(message diag.Message) bool {
	if message.Resource == nil {
		return false
	}
	if message.Resource.Metadata.Labels[sidecarInjectLabel] == "true" {
		return true
	}
	return message.Resource.Metadata.Annotations[sidecarInjectLabel] == "true"
}

func isMutedSystemNamespace(ns string) bool {
	if strings.HasPrefix(ns, deckhouseSystemNamespacePrefix) {
		return true
	}
	for _, systemNS := range mutedSystemNamespaces {
		if ns == systemNS {
			return true
		}
	}
	return false
}

func messageLabels(message diag.Message, revision string) (messageType, namespace, resourceName, severity, code, messageText string) {
	code = message.Type.Code()
	messageType = messageTypeName(message.Type)
	severity = message.Type.Level().String()
	namespace = "_cluster"
	resourceName = "_none"
	messageText = truncateLabelValue(fmt.Sprintf(message.Type.Template(), message.Parameters...))

	if message.Resource != nil {
		if ns := string(message.Resource.Metadata.FullName.Namespace); ns != "" {
			namespace = ns
		}
		if origin := message.Resource.Origin.FriendlyName(); origin != "" {
			resourceName = origin
		} else {
			id := message.Resource.Metadata.FullName
			resourceName = fmt.Sprintf("%s/%s", id.Name, id.Namespace)
		}
	}

	return messageType, namespace, resourceName, severity, code, messageText
}

const maxMessageLabelLen = 256

func truncateLabelValue(value string) string {
	if len(value) <= maxMessageLabelLen {
		return value
	}
	return value[:maxMessageLabelLen-3] + "..."
}

func waitForNextRun(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
