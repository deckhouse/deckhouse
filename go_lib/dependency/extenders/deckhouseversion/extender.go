/*
Copyright 2024 Flant JSC

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

package deckhouseversion

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	scherror "github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders/error"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "DeckhouseVersion"
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	logger         *log.Logger
	versionMatcher *versionmatcher.Matcher
	err            error
}

const TestExtenderDeckhouseVersionEnv = "TEST_EXTENDER_DECKHOUSE_VERSION"

func NewExtender(deckhouseVersion string, logger *log.Logger) *Extender {
	extender := &Extender{
		logger:         logger,
		versionMatcher: versionmatcher.New(false),
	}

	deckhouseVersion = strings.TrimSpace(deckhouseVersion)

	if val := os.Getenv(TestExtenderDeckhouseVersionEnv); val != "" {
		parsed, err := semver.NewVersion(val)
		if err == nil {
			extender.logger.Debug("setting deckhouse version from env", slog.String("version", parsed.String()))
			extender.versionMatcher.ChangeBaseVersion(parsed)

			return extender
		}

		extender.logger.Warn("cannot parse version variable value, env: "+TestExtenderDeckhouseVersionEnv, slog.String("value", val), log.Err(err))
	}

	if deckhouseVersion == "dev" {
		extender.logger.Warn("this is dev cluster, default version will be used", slog.String("version", extender.versionMatcher.GetBaseVersion().Original()))

		return extender
	}

	parsed, err := semver.NewVersion(deckhouseVersion)
	if err == nil {
		// TODO: change workflow with semver
		// mastermind lib has a problem to compare pre-released versions
		// we must to trim them to compare release version
		sanitizedVersion, err := removePrereleaseAndMetadata(parsed)
		if err != nil {
			extender.logger.Warn("cannot remove pre-release tag or metadata, default version will be used", slog.String("version", extender.versionMatcher.GetBaseVersion().Original()))

			return extender
		}

		extender.logger.Debug("setting deckhouse version from file", slog.String("version", sanitizedVersion.String()))
		extender.versionMatcher.ChangeBaseVersion(sanitizedVersion)

		return extender
	}

	extender.logger.Warn("failed to parse deckhouse version")
	extender.err = err

	return extender
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Warn("adding installed constraint for module failed", slog.String("name", name), slog.String("constraint", rawConstraint), log.Err(err))

		return fmt.Errorf("add constraint: %w", err)
	}

	e.logger.Debug("installed constraint for module is added", slog.String("name", name))

	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	e.logger.Debug("deleting installed constraint for module", slog.String("name", name))

	e.versionMatcher.DeleteConstraint(name)
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return true
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	if !e.versionMatcher.Has(name) {
		return nil, nil
	}

	if e.err != nil {
		e.logger.Warn("parse deckhouse version failed", log.Err(e.err))

		return nil, &scherror.PermanentError{Err: fmt.Errorf("parse deckhouse version failed: %s", e.err)}
	}

	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Warn("requirements of module are not satisfied: current deckhouse version is not suitable", slog.String("name", name), log.Err(err))

		return ptr.To(false), fmt.Errorf("requirements are not satisfied: current deckhouse version is not suitable: %s", err.Error())
	}

	e.logger.Debug("requirements of module are satisfied", slog.String("name", name))

	return ptr.To(true), nil
}

func (e *Extender) ValidateBaseVersion(baseVersion string) (string, error) {
	if name, err := e.versionMatcher.ValidateBaseVersion(baseVersion); err != nil {
		if name != "" {
			e.logger.Warn("requirements of module are not satisfied; deckhouse version is not suitable", slog.String("name", name), slog.String("version", baseVersion), log.Err(err))

			return name, fmt.Errorf("requirements of the '%s' module are not satisfied: %s deckhouse version is not suitable: %s", name, baseVersion, err.Error())
		}

		e.logger.Warn("modules requirements cannot be checked, deckhouse version is invalid", slog.String("version", baseVersion), log.Err(err))

		return "", fmt.Errorf("modules requirements cannot be checked: deckhouse version is invalid: %s", err.Error())
	}

	e.logger.Debug("modules requirements for deckhouse version are satisfied", slog.String("version", baseVersion))

	return "", nil
}

func (e *Extender) ValidateRelease(releaseName, rawConstraint string) error {
	if e.err != nil {
		return fmt.Errorf("parse deckhouse version failed: %s", e.err)
	}

	if err := e.versionMatcher.ValidateConstraint(rawConstraint); err != nil {
		e.logger.Warn("requirements of module release are not satisfied: current deckhouse version is not suitable", slog.String("name", releaseName), log.Err(err))

		return fmt.Errorf("requirements are not satisfied: current deckhouse version is not suitable: %s", err.Error())
	}

	e.logger.Debug("requirements of module release are satisfied", slog.String("name", releaseName))

	return nil
}

// removePrereleaseAndMetadata returns a version without prerelease and metadata parts
func removePrereleaseAndMetadata(version *semver.Version) (*semver.Version, error) {
	if len(version.Prerelease()) > 0 {
		woPrerelease, err := version.SetPrerelease("")
		if err != nil {
			return nil, fmt.Errorf("set prerelease: %w", err)
		}

		version = &woPrerelease
	}

	if len(version.Metadata()) > 0 {
		woMetadata, err := version.SetMetadata("")
		if err != nil {
			return nil, fmt.Errorf("set metadata: %w", err)
		}

		version = &woMetadata
	}

	return version, nil
}
