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

package crdmigration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
)

var newSystemTypeSpelling = map[deckhousev1.SystemType]deckhousev1.SystemType{
	deckhousev1.SystemTypeLegacyMutable:   deckhousev1.SystemTypeGenericOS,
	deckhousev1.SystemTypeLegacyImmutable: deckhousev1.SystemTypeDeckhouseEngine,
}

func MigrateSystemTypes(ctx context.Context, c client.Client) error {
	var errs []error

	groups := &deckhousev1.NodeGroupList{}
	if err := c.List(ctx, groups); err != nil {
		errs = append(errs, fmt.Errorf("list NodeGroups: %w", err))
	}
	for i := range groups.Items {
		group := &groups.Items[i]
		systemType, legacy := newSystemTypeSpelling[group.Spec.SystemType]
		if !legacy {
			continue
		}
		patch := client.MergeFromWithOptions(group.DeepCopy(), client.MergeFromWithOptimisticLock{})
		group.Spec.SystemType = systemType
		if err := c.Patch(ctx, group, patch); err != nil {
			errs = append(errs, fmt.Errorf("migrate NodeGroup %s: %w", group.Name, err))
		}
	}

	nodes := &corev1.NodeList{}
	legacyLabel := client.MatchingLabels{common.NodeSystemTypeLabel: string(deckhousev1.SystemTypeLegacyImmutable)}
	if err := c.List(ctx, nodes, legacyLabel); err != nil {
		errs = append(errs, fmt.Errorf("list Nodes: %w", err))
	}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		patch := client.MergeFromWithOptions(node.DeepCopy(), client.MergeFromWithOptimisticLock{})
		node.Labels[common.NodeSystemTypeLabel] = string(deckhousev1.SystemTypeDeckhouseEngine)
		if err := c.Patch(ctx, node, patch); err != nil {
			errs = append(errs, fmt.Errorf("migrate Node %s: %w", node.Name, err))
		}
	}

	return errors.Join(errs...)
}

func RetrySystemTypeMigration(ctx context.Context, c client.Client, logger logr.Logger, interval time.Duration) {
	_ = wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		if err := MigrateSystemTypes(ctx, c); err != nil {
			if ctx.Err() == nil {
				logger.Info("the legacy systemType spelling is not migrated yet, retrying", "error", err.Error())
			}
			return false, nil
		}
		return true, nil
	})
}
