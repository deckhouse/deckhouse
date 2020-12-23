package resources

import (
	"strings"

	"github.com/ghodss/yaml"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

// ControllerMustHaveVPAAndPDB fills linting error regarding VPA
// TODO (@evgeny.shevchenko): ... and PDB
func ControllerMustHaveVPAAndPDB(module types.Module, values string, objectStore *storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList) {
	if !isVPAEnabled(values) {
		return
	}

	scope := newLintingScope(objectStore, lintRuleErrorsList)

	vpaTargets, vpaTolerationGroups := parseTargetsAndTolerationGroups(scope)

	for index, object := range scope.Objects() {
		// Skip non-pod controllers and modules which control VPA themselves
		if !isPodController(object.Unstructured.GetKind()) || shouldSkipModuleResource(module.Name, &index) {
			continue
		}

		if !ensureVPAIsPresent(scope, vpaTargets, index, object) {
			continue
		}

		// TODO (@evgeny.shevchenko): check for PDBs here

		if !ensureContainersWithoutRequests(scope, object) {
			continue
		}

		ensureTolerations(scope, vpaTolerationGroups, index, object)
	}
}

func isVPAEnabled(values string) bool {
	var v struct {
		Global struct{ EnabledModules []string }
	}
	err := yaml.Unmarshal([]byte(values), &v)
	if err != nil {
		panic("unable to parse global.enabledModules values section")
	}

	for _, module := range v.Global.EnabledModules {
		if module == "vertical-pod-autoscaler-crd" {
			return true
		}
	}
	return false
}

func isPodController(kind string) bool {
	return kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet"
}

func shouldSkipModuleResource(moduleName string, r *storage.ResourceIndex) bool {
	switch moduleName {
	// Controllers VPA is configured through cr settings
	case "ingress-nginx":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-ingress-nginx" && strings.HasPrefix(r.Name, "controller-")

	// Network gateway snat daemonset tolerations is configured through module values
	case "network-gateway":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-network-gateway" && r.Name == "snat"

	// Metal LB speaker daemonset tolerations is configured through module values
	case "metallb":
		return r.Kind == "DaemonSet" && r.Namespace == "d8-metallb" && r.Name == "speaker"

	default:
		return false
	}
}
