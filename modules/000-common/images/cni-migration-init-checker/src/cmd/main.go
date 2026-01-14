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

var (
	cniMigrationGVR = schema.GroupVersionResource{
		Group:    "network.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "cnimigrations",
	}
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("Error executing run", "error", err)
		os.Exit(1)
	}
}

func run() error {
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

	// 1. List CNIMigrations to find if any exists.
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

	myCNI := os.Getenv("CNI_NAME")
	// Parse CNI_NAME as a comma-separated list
	allowedCNIs := make(map[string]bool)
	if myCNI != "" {
		for _, c := range strings.Split(myCNI, ",") {
			allowedCNIs[strings.TrimSpace(c)] = true
		}
	}

	// 2. Watch/Poll the CNIMigration resource.
	migrationName := list.Items[0].GetName()
	slog.Info("Found CNIMigration. Checking status...", "name", migrationName, "myCNI", myCNI)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			obj, err := dynamicClient.Resource(cniMigrationGVR).Get(ctx, migrationName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					slog.Info("CNIMigration resource deleted. Starting normally.")
					return nil
				}
				slog.Error("Error getting CNIMigration. Retrying...", "error", err)
				continue
			}

			// Check status
			status, found, err := unstructured.NestedMap(obj.Object, "status")
			if err != nil || !found {
				slog.Info("Status not found in CNIMigration. Waiting...")
				continue
			}

			// Check if we are one of the current CNIs
			currentCNI, found, _ := unstructured.NestedString(status, "currentCNI")
			if found && allowedCNIs[currentCNI] {
				slog.Info("Current CNI matches one of the allowed CNIs. Starting agent.", "current_cni", currentCNI)
				return nil
			}

			// Check conditions
			conditions, found, err := unstructured.NestedSlice(status, "conditions")
			if err != nil || !found {
				slog.Info("Conditions not found in CNIMigration status. Waiting...")
				continue
			}

			nodeCleanupSucceeded := false
			for _, c := range conditions {
				cond, ok := c.(map[string]any)
				if !ok {
					continue
				}
				typeStr, _, _ := unstructured.NestedString(cond, "type")
				statusStr, _, _ := unstructured.NestedString(cond, "status")

				if typeStr == "NodeCleanupSucceeded" && statusStr == "True" {
					nodeCleanupSucceeded = true
					break
				}
			}

			if nodeCleanupSucceeded {
				slog.Info("NodeCleanupSucceeded is True. Starting agent.")
				return nil
			}

			slog.Info("Waiting for NodeCleanupSucceeded...")
		}
	}
}
