package update

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
	Disruptions   []string
	ApplyAfter    *time.Time
	CooldownUntil *time.Time

	Status v1alpha1.DeckhouseReleaseStatus // don't set transition time here to avoid snapshot overload
}

type byVersion []DeckhouseRelease

func (a byVersion) Len() int {
	return len(a)
}
func (a byVersion) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a byVersion) Less(i, j int) bool {
	return a[i].Version.LessThan(a[j].Version)
}

type statusPatch v1alpha1.DeckhouseReleaseStatus

func (sp statusPatch) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"status": v1alpha1.DeckhouseReleaseStatus(sp),
	}

	return json.Marshal(m)
}
