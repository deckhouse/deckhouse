/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func NewCleaner(patcher go_hook.PatchCollector, logger go_hook.Logger, pods []snapshot.Pod) Cleaner {
	return &kubeCleaner{
		pods:                          pods,
		persistenceVolumeClaimDeleter: newPersistentVolumeClaimDeleter(patcher, logger),
		statefulSetDeleter:            newStatefulSetDeleter(patcher, logger),
	}
}

type Cleaner interface {
	Clean(string, *XState, *XState)
}

type kubeCleaner struct {
	pods []snapshot.Pod

	persistenceVolumeClaimDeleter Deleter
	statefulSetDeleter            Deleter
}

// Clean deletes kubernetes resources that prevent further progress
func (c *kubeCleaner) Clean(x string, curSts, newSts *XState) {
	if !curSts.scheduled() {
		// Nothing to clean
		return
	}

	var (
		pod       snapshot.Pod
		podExists bool
	)
	for _, p := range c.pods {
		if p.Index == x {
			pod, podExists = p, true
			break
		}
	}

	var (
		zoneChanged         = curSts.Zone != newSts.Zone
		storageClassChanged = curSts.StorageClass != newSts.StorageClass

		// We have to re-create the StatefulSet because
		//  - `volumeClaimTemplates` field is read-only and kube-apiserver will not accept the update;
		//  - if nothing changed for the StatefulSet and PVC, we should not tolerate failing pod [1];
		//  - if something changed while the pod is not running, we should take care of the pod specifically [1],
		//    see https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback
		//
		//  [1] We cannot just delete the pod. If we do, kube controller manager can re-create it before statefulset
		//      will be updated by Helm. So we avoid the race by re-creating the StatefulSet comlletely.
		shouldClean = storageClassChanged || zoneChanged || (podExists && !pod.Ready)
	)

	if !shouldClean {
		return
	}

	if curSts.StorageClass != snapshot.DefaultStorageClass {
		// If we use emptyDir, we don't have PVC
		c.persistenceVolumeClaimDeleter.Delete(snapshot.Index(x).PersistenceVolumeClaimName())
	}

	c.statefulSetDeleter.Delete(snapshot.Index(x).StatefulSetName())
}
