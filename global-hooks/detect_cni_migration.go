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

package hooks

import (
	"context"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/detect-cni-migration",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cni_migrations",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "CNIMigration",
			FilterFunc: applyCNIMigrationFilter,
		},
	},
}, detectCNIMigration)

type CNIMigrationInfo struct {
	Name        string
	Created     time.Time
	IsSucceeded bool
}

func applyCNIMigrationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	isSucceeded := false
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			typeStr, _ := cond["type"].(string)
			statusStr, _ := cond["status"].(string)
			if typeStr == "Succeeded" && statusStr == "True" {
				isSucceeded = true
				break
			}
		}
	}

	return CNIMigrationInfo{
		Name:        obj.GetName(),
		Created:     obj.GetCreationTimestamp().Time,
		IsSucceeded: isSucceeded,
	}, nil
}

func detectCNIMigration(_ context.Context, input *go_hook.HookInput) error {
	snapshots := input.Snapshots.Get("cni_migrations")

	if len(snapshots) == 0 {
		input.Values.Remove("global.internal.cniMigrationEnabled")
		input.Values.Remove("global.internal.cniMigrationName")
		input.Values.Remove("global.internal.cniMigrationWebhooksDisable")
		return nil
	}

	// Find the oldest migration (Active)
	var activeMigration CNIMigrationInfo
	found := false

	for _, s := range snapshots {
		var info CNIMigrationInfo
		if err := s.UnmarshalTo(&info); err != nil {
			continue
		}

		if !found || info.Created.Before(activeMigration.Created) {
			activeMigration = info
			found = true
		}
	}

	if !found {
		return nil
	}

	input.Values.Set("global.internal.cniMigrationName", activeMigration.Name)
	input.Values.Set("global.internal.cniMigrationEnabled", true)

	// Check if it is finished (Succeeded condition is True)
	if activeMigration.IsSucceeded {
		// Migration is done. Re-enable external webhooks by removing the ignore flag.
		input.Values.Remove("global.internal.cniMigrationWebhooksDisable")
	} else {
		// Migration is in progress. Disable external validating/mutating webhooks
		// to prevent them from blocking pod restarts or other migration actions.
		input.Values.Set("global.internal.cniMigrationWebhooksDisable", true)
	}

	return nil
}
