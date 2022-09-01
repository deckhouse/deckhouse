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

package updater

import (
	"encoding/json"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/apis/v1alpha1"
)

type DeckhouseRelease struct {
	Name    string
	Version *semver.Version

	ManuallyApproved                bool
	HasSuspendAnnotation            bool
	HasForceAnnotation              bool
	HasDisruptionApprovedAnnotation bool

	Requirements  map[string]string
	ChangelogLink string
	Disruptions   []string
	ApplyAfter    *time.Time
	CooldownUntil *time.Time

	Status v1alpha1.DeckhouseReleaseStatus // don't set transition time here to avoid snapshot overload
}

type ByVersion []DeckhouseRelease

func (a ByVersion) Len() int {
	return len(a)
}
func (a ByVersion) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a ByVersion) Less(i, j int) bool {
	return a[i].Version.LessThan(a[j].Version)
}

type StatusPatch v1alpha1.DeckhouseReleaseStatus

func (sp StatusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.DeckhouseReleaseStatus(sp),
	}

	return json.Marshal(m)
}

type DeckhouseReleaseData struct {
	IsUpdating bool
	Notified   bool
}
