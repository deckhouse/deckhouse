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

//go:build deckhouse_external

package main

import (
	"context"
	"fmt"
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

const outputThreshold = diag.Info

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
		if message.Type.Level().IsWorseThanOrEqualTo(outputThreshold) {
			messages = append(messages, message)
		}
	}

	log.Infof("analysis completed: revision=%s messages=%d", revision, len(messages))
	return messages, nil
}

func messageLabels(message diag.Message, revision string) (messageType, namespace, resourceName, severity, code string) {
	code = message.Type.Code()
	messageType = messageTypeName(message.Type)
	severity = message.Type.Level().String()
	namespace = "_cluster"
	resourceName = "_none"

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

	return messageType, namespace, resourceName, severity, code
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
