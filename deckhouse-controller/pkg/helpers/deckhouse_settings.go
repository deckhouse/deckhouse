package helpers

import (
	"sync"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

// DeckhouseSettings is an openapi spec for deckhouse settings, it's not a part of DeckhouseReleaseSpec but rather
// it's a part of DeckhouseReleaseController
type DeckhouseSettings struct {
	Update struct {
		Mode                   string                     `json:"mode"`
		DisruptionApprovalMode string                     `json:"disruptionApprovalMode"`
		Windows                update.Windows             `json:"windows"`
		NotificationConfig     updater.NotificationConfig `json:"notification"`
	} `json:"update"`
	ReleaseChannel string `json:"releaseChannel"`
}

func NewDeckhouseSettingsContainer(spec *DeckhouseSettings) *DeckhouseSettingsContainer {
	return &DeckhouseSettingsContainer{spec: spec}
}

type DeckhouseSettingsContainer struct {
	spec *DeckhouseSettings
	lock sync.Mutex
}

func (c *DeckhouseSettingsContainer) Set(settings *DeckhouseSettings) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.spec.ReleaseChannel = settings.ReleaseChannel
	c.spec.Update.Mode = settings.Update.Mode
	c.spec.Update.Windows = settings.Update.Windows
	c.spec.Update.DisruptionApprovalMode = settings.Update.DisruptionApprovalMode
	c.spec.Update.NotificationConfig = settings.Update.NotificationConfig
}

func (c *DeckhouseSettingsContainer) Get() *DeckhouseSettings {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.spec
}
