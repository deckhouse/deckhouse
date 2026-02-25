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
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var cniMigrationGVR = schema.GroupVersionResource{
	Group:    "network.deckhouse.io",
	Version:  "v1alpha1",
	Resource: "cnimigrations",
}

var cniNodeMigrationGVR = schema.GroupVersionResource{
	Group:    "network.deckhouse.io",
	Version:  "v1alpha1",
	Resource: "cninodemigrations",
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("Error executing run", "error", err)
		os.Exit(1)
	}
}

func run() error {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return fmt.Errorf("NODE_NAME environment variable is not set")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig for local development
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			return fmt.Errorf("unable to get in-cluster config and KUBECONFIG is not set: %w", err)
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("unable to build config from flags: %w", err)
		}
	}

	slog.Info("Kubernetes connection details", "host", config.Host)

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("unable to create dynamic client: %w", err)
	}

	ctx := context.Background()
	localCNIName := os.Getenv("CNI_NAME")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// First attempt immediately
	done, err := checkStatus(ctx, dynamicClient, nodeName, localCNIName)
	if err != nil {
		slog.Error("Check failed (will retry)", "error", err)
	} else if done {
		return nil
	}

	// Retry loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			done, err := checkStatus(ctx, dynamicClient, nodeName, localCNIName)
			if err != nil {
				slog.Error("Check failed (will retry)", "error", err)
				continue
			}
			if done {
				return nil
			}
		}
	}
}

// checkStatus checks if the migration is complete for the node or if the agent can start safely.
func checkStatus(ctx context.Context, client dynamic.Interface, nodeName, localCNIName string) (bool, error) {
	// Check if migration is active.
	list, err := client.Resource(cniMigrationGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			slog.Info("CNIMigration resource not found (CRD might be missing). Starting normally.")
			return true, nil
		}
		return false, fmt.Errorf("failed to list CNIMigrations: %w", err)
	}

	if len(list.Items) == 0 {
		slog.Info("No CNIMigration resources found. Starting normally.")
		return true, nil
	}

	activeMigration := list.Items[0]

	for _, item := range list.Items[1:] {
		itemCreated := item.GetCreationTimestamp().UnixNano()
		activeCreated := activeMigration.GetCreationTimestamp().UnixNano()

		isOlder := itemCreated < activeCreated
		isSameTimeButSmallerName := itemCreated == activeCreated && item.GetName() < activeMigration.GetName()

		if isOlder || isSameTimeButSmallerName {
			activeMigration = item
		}
	}

	// Allow the old CNI to run until it is explicitly disabled.
	status, found, _ := unstructured.NestedMap(activeMigration.Object, "status")
	if found {
		currentCNI, ok, _ := unstructured.NestedString(status, "currentCNI")
		if ok && localCNIName != "" {
			for c := range strings.SplitSeq(localCNIName, ",") {
				if strings.TrimSpace(c) == currentCNI {
					slog.Info("Current CNI matches my CNI. Starting agent.", "current_cni", currentCNI)
					return true, nil
				}
			}
		}
	}

	// Wait for node cleanup confirmation.
	obj, err := client.Resource(cniNodeMigrationGVR).Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			slog.Info("CNINodeMigration not found yet. Waiting", "node", nodeName)
			return false, nil
		}
		return false, fmt.Errorf("error getting CNINodeMigration: %w", err)
	}

	status, found, err = unstructured.NestedMap(obj.Object, "status")
	if err != nil || !found {
		return false, nil // Status not populated yet.
	}

	conditions, found, err := unstructured.NestedSlice(status, "conditions")
	if err != nil || !found {
		return false, nil // Conditions not populated yet.
	}

	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		typeStr, _, _ := unstructured.NestedString(cond, "type")
		statusStr, _, _ := unstructured.NestedString(cond, "status")

		if typeStr == "CleanupDone" && statusStr == "True" {
			slog.Info("Node cleanup complete. Starting agent.")
			return true, nil
		}
	}

	slog.Info("Waiting for CleanupDone condition on CNINodeMigration")
	return false, nil
}
