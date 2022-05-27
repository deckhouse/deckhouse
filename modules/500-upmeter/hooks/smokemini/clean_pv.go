// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smokemini

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// Delete obsolete upmeter PV if they are stuck with reclaim retention policy or by any other reason.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "clean_obsolete_pv",
			Crontab: "33 * * * *",
		},
	},
}, dependency.WithExternalDependencies(func(input *go_hook.HookInput, dc dependency.Container) error {
	k := dc.MustGetK8sClient()
	return removeSmokeMiniPersistentVolumes(context.TODO(), k)
}))

func removeSmokeMiniPersistentVolumes(ctx context.Context, k k8s.Client) error {
	pvs, err := k.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing PVs: %v", err)
	}

	limit := 33 // being gentle
	for _, pv := range pvs.Items {
		if name, ok := shouldDeletePV(&pv); ok && limit > 0 {
			err = k.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("deleting PV %s: %v", name, err)
			}
			limit--
		}
	}

	return nil
}

func shouldDeletePV(pv *v1.PersistentVolume) (string, bool) {
	// Skip ones which are already terminating
	if pv.GetDeletionTimestamp() != nil {
		return "", false
	}

	// Skip ones not from smoke-mini
	if pv.Spec.ClaimRef == nil {
		return "", false
	}
	pvcName := pv.Spec.ClaimRef.Name
	if !strings.HasPrefix(pvcName, "disk-smoke-mini-") {
		return "", false
	}

	return pv.GetName(), true
}
