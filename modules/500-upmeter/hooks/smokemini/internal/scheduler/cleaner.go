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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func NewCleaner(patcher *object_patch.PatchCollector, logger *logrus.Entry, pods []snapshot.Pod) Cleaner {
	return &kubeCleaner{
		pods:       pods,
		podDeleter: NewPodDeleter(patcher, logger),
		pvcDeleter: newPersistentVolumeClaimDeleter(patcher, logger),
		stsDeleter: newStatefulSetDeleter(patcher, logger),
	}
}

type Cleaner interface {
	Clean(string, *XState, *XState)
}

type kubeCleaner struct {
	pods []snapshot.Pod

	podDeleter Deleter
	pvcDeleter Deleter
	stsDeleter Deleter
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

		deletePVC = storageClassChanged || zoneChanged

		// We have to re-create the StatefulSet because `volumeClaimTemplates` field is read-only and
		// kube-apiserver will not accept the update
		deleteSTS = storageClassChanged

		// - If nothing changed for the StatefulSet and PVC, we should not tolerate failing pod.
		// - If something changed while the pod is not running, we should take care of the pod specifically.
		//   See https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback
		deletePod = deleteSTS || deletePVC || (podExists && !pod.Ready)
	)

	if deleteSTS {
		c.stsDeleter.Delete(snapshot.Index(x).StatefulSetName())
	}

	if deletePVC {
		c.pvcDeleter.Delete(snapshot.Index(x).PersistenceVolumeClaimName())
	}

	if deletePod {
		c.podDeleter.Delete(snapshot.Index(x).PodName())
	}
}
