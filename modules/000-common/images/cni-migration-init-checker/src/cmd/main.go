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

	// 1. Check if CNIMigration exists. If not, we are not migrating.
	list, err := dynamicClient.Resource(cniMigrationGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			slog.Info("CNIMigration resource not found (CRD might be missing). Assuming no migration.")
			return nil
		}
		return fmt.Errorf("failed to list CNIMigrations: %w", err)
	}

	if len(list.Items) == 0 {
		slog.Info("No CNIMigration resources found. Starting normally.")
		return nil
	}

	cniMigration := list.Items[0]
	localCNIName := os.Getenv("CNI_NAME")

	// Check if we are the CurrentCNI (allow old CNI to run until it is killed)
	status, found, _ := unstructured.NestedMap(cniMigration.Object, "status")
	if found {
		currentCNI, found, _ := unstructured.NestedString(status, "currentCNI")
		if found && localCNIName != "" {
			// Check if localCNIName is in the allowed list (comma separated)
			for c := range strings.SplitSeq(localCNIName, ",") {
				if strings.TrimSpace(c) == currentCNI {
					slog.Info("Current CNI matches my CNI. Starting agent.", "current_cni", currentCNI)
					return nil
				}
			}
		}
	}

	slog.Info("Migration in progress. Waiting for node cleanup...", "node", nodeName)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check global migration again to fail-fast if it is deleted
			_, err := dynamicClient.Resource(cniMigrationGVR).Get(ctx, cniMigration.GetName(), metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					slog.Info("CNIMigration deleted. Starting.")
					return nil
				}
				slog.Error("Error checking CNIMigration", "error", err)
			}

			// Check CNINodeMigration
			obj, err := dynamicClient.Resource(cniNodeMigrationGVR).Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					slog.Info("CNINodeMigration not found yet. Waiting...")
					continue
				}
				slog.Error("Error getting CNINodeMigration. Retrying...", "error", err)
				continue
			}

			status, found, err := unstructured.NestedMap(obj.Object, "status")
			if err != nil || !found {
				continue
			}

			conditions, found, err := unstructured.NestedSlice(status, "conditions")
			if err != nil || !found {
				continue
			}

			for _, c := range conditions {
				cond, ok := c.(map[string]any)
				if !ok {
					continue
				}
				typeStr, _, _ := unstructured.NestedString(cond, "type")
				statusStr, _, _ := unstructured.NestedString(cond, "status")

				// Check for CleanupDone == True
				if typeStr == "CleanupDone" && statusStr == "True" {
					slog.Info("Node cleanup complete. Starting agent.")
					return nil
				}
			}

			slog.Info("Waiting for CleanupDone condition on CNINodeMigration...")
		}
	}
}
