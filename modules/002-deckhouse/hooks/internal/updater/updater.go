/*
Copyright 2022 Flant JSC

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

package d8updater

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/deckhouse/deckhouse/go_lib/updater"
)

func NewDeckhouseUpdater(input *go_hook.HookInput, mode string, data updater.DeckhouseReleaseData, podIsReady, isBootstrapping bool) (*updater.Updater[*DeckhouseRelease], error) {
	return updater.NewUpdater[*DeckhouseRelease](input, mode, data, podIsReady, isBootstrapping, newReleaseUpdater(input), newMetricsUpdater(input))
}

func newReleaseUpdater(input *go_hook.HookInput) *releaseUpdater {
	return &releaseUpdater{input.PatchCollector}
}

type releaseUpdater struct {
	patchCollector *object_patch.PatchCollector
}

func (ru *releaseUpdater) UpdateStatus(release any, msg, phase string) {
	r, ok := release.(*DeckhouseRelease)
	if !ok {
		panic(fmt.Sprintf("Unexpected type %T", release))
	}

	st := StatusPatch{
		Phase:          phase,
		Message:        msg,
		Approved:       r.Status.Approved,
		TransitionTime: time.Now().UTC(),
	}
	ru.patchCollector.MergePatch(st, "deckhouse.io/v1alpha1", "DeckhouseRelease", "", r.Name, object_patch.WithSubresource("/status"))

	r.Status.Phase = phase
	r.Status.Message = msg
}
