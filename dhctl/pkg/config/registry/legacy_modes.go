/*
Copyright 2026 Flant JSC

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

package registry

import (
	"errors"
	"fmt"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/moduleconfig"
)

// RefusePreviousImplementationModes refuses `Proxy` and `Local` written by the user into the deckhouse
// ModuleConfig.
//
// Both are the previous implementation's in-cluster registries, and that implementation cannot be
// installed from this release: nothing in the cluster renders it, so an installation that accepted
// either mode ran to the end and then waited about half an hour for a state nobody writes any more.
//
// It has to run on the settings as the user wrote them, before FoldLegacyDirectIntoInit and
// BundleBootstrapInputs.Resolve. Resolve expresses an installation from a bundle as `Local` too, and
// that one is the current implementation; it only ever fills in settings the user left empty, so a
// check placed here never sees it.
//
// It refuses in every operation, commander's converge, check and destroy of an existing cluster
// included. dhctl has no code for these modes any more, and reading them as something else, such as
// `Unmanaged` or the upstream folded into InitConfiguration, would quietly change how that cluster's
// nodes pull. Resources that still name them are corrected first, and ExistingClusterRefusal tells
// commander's operations how.
//
// `Direct` and `Unmanaged` are left alone: FoldLegacyDirectIntoInit reads the first as the registry
// address, and the second already means a cluster whose registry nobody manages.
func RefusePreviousImplementationModes(settings *module_config.DeckhouseSettings) error {
	if settings == nil {
		return nil
	}

	switch settings.Mode {
	case constant.ModeProxy, constant.ModeLocal:
		return &PreviousImplementationModeError{Mode: settings.Mode}
	}

	return nil
}

// RefusePreviousImplementationModesInModuleConfig is RefusePreviousImplementationModes over the
// registry settings of one deckhouse ModuleConfig, given as YAML or JSON.
//
// It is for the callers that have to look at every copy of that ModuleConfig a configuration
// carries, and not only at the one the installation reads its registry from: see
// MetaConfig.prepareRegistry in the config package.
func RefusePreviousImplementationModesInModuleConfig(deckhouseMC []byte) error {
	settings, err := ParseYAMLDeckhouseMC(deckhouseMC)
	if err != nil {
		return fmt.Errorf("parse the registry settings of the deckhouse ModuleConfig: %w", err)
	}

	return RefusePreviousImplementationModes(settings)
}

// PreviousImplementationModeError is the refusal of RefusePreviousImplementationModes. Its message
// is written for a new installation; commander's check, converge and destroy put it the way an
// existing cluster needs it with ExistingClusterRefusal.
type PreviousImplementationModeError struct {
	Mode constant.ModeType
}

func (e *PreviousImplementationModeError) Error() string {
	return fmt.Sprintf(
		"registry.mode %q in the deckhouse ModuleConfig belongs to the previous registry implementation, "+
			"which cannot be installed from this release. Remove settings.registry from the deckhouse "+
			"ModuleConfig and configure the registry ModuleConfig: mode: Managed with primary.upstream "+
			"(add storage.cache: true for an in-cluster cache); for an air-gapped install, mode: Managed "+
			"with storage.cache, storage.source and no primary.upstream, passing the bundle with "+
			"--img-bundle-path; or keep the module Unmanaged and name the registry in "+
			"InitConfiguration.deckhouse",
		e.Mode,
	)
}

// ExistingClusterRefusal returns the refusal of RefusePreviousImplementationModes found in err as
// commander's check, converge and destroy have to put it, and any other err unchanged.
//
// Those operations parse the resources an existing cluster was installed with. The text for a new
// installation tells the user how to write a configuration from scratch, which is not what they
// face: it is the registry settings of the stored resources that are out of date, and they are
// brought in line with the cluster. Copying the cluster's whole deckhouse ModuleConfig is not
// enough: a cluster already switched to the registry module can still store Proxy or Local there,
// with no effect, and the copy would be refused again. Only the spec is copied, never the
// server-set metadata and status that -o yaml prints.
func ExistingClusterRefusal(err error) error {
	var refusal *PreviousImplementationModeError
	if !errors.As(err, &refusal) {
		return err
	}

	return fmt.Errorf(
		"registry.mode %q in the deckhouse ModuleConfig of this cluster's resources belongs to the previous "+
			"registry implementation, which this release does not manage, so the resources are refused and "+
			"nothing is changed. Bring the registry settings of the resources in line with the cluster and "+
			"repeat the operation. In the deckhouse ModuleConfig of the resources, set settings.registry to "+
			"the cluster's own (d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry}') if that sets "+
			"mode Direct or Unmanaged; otherwise remove settings.registry, also when the cluster's own still "+
			"names Proxy or Local: on a cluster that has a registry ModuleConfig that value is left over and "+
			"has no effect. If the cluster has a registry ModuleConfig, add one with the same spec to the "+
			"resources (d8 k get mc registry -o jsonpath='{.spec}'). A cluster that has no registry "+
			"ModuleConfig and still sets %q in its deckhouse ModuleConfig runs the previous implementation: "+
			"migrate it to the registry module first, on the release it runs now",
		refusal.Mode, refusal.Mode,
	)
}
