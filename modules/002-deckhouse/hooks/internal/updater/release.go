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
