package hooks

import (
	"errors"

	"github.com/blang/semver"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func init() {
	f := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		desiredVersion, err := semver.Parse(requirementValue)
		if err != nil {
			return false, err
		}

		currentVersionStr := getter.Get("global.discovery.kubernetesVersion").String()
		currentVersion, err := semver.Parse(currentVersionStr)
		if err != nil {
			return false, err
		}

		if currentVersion.GE(desiredVersion) {
			return true, nil
		}

		return false, errors.New("current kubernetes version is lower then required")
	}

	requirements.Register("k8s", f)
}
