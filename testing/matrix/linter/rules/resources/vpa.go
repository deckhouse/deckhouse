package resources

import (
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

var exceptions = map[string]func(r *storage.ResourceIndex) bool{
	// Controllers VPA is configured through cr settings
	"ingress-nginx": func(r *storage.ResourceIndex) bool {
		if r.Kind == "DaemonSet" && r.Namespace == "d8-ingress-nginx" && strings.HasPrefix(r.Name, "controller-") {
			return true
		}
		return false
	},
}

func ControllerMustHasVPA(m types.Module, objectStore storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList) errors.LintRuleError {
	exceptionFunc := exceptions[m.Name]
	if exceptionFunc == nil {
		exceptionFunc = func(r *storage.ResourceIndex) bool { return false }
	}

	vpaTargets := make(map[storage.ResourceIndex]struct{})

	for index, object := range objectStore.Storage {
		objectKind := object.Unstructured.GetKind()
		if objectKind != "VerticalPodAutoscaler" {
			continue
		}

		r := storage.ResourceIndex{}
		r.Namespace = index.Namespace

		specs, ok := object.Unstructured.Object["spec"].(map[string]interface{})
		if !ok {
			lintRuleErrorsList.Add(errors.NewLintRuleError(
				"VPA005",
				object.Identity(),
				false,
				"No VPA specs is found for object",
			))
			continue
		}

		refsFromSpec, ok := specs["targetRef"].(map[string]interface{})
		if !ok {
			lintRuleErrorsList.Add(errors.NewLintRuleError(
				"VPA005",
				object.Identity(),
				false,
				"No VPA specs targetRef is found for object",
			))
			continue
		}

		r.Name = refsFromSpec["name"].(string)
		r.Kind = refsFromSpec["kind"].(string)

		vpaTargets[r] = struct{}{}
	}

	for index, object := range objectStore.Storage {
		if exceptionFunc(&index) {
			continue
		}

		switch object.Unstructured.GetKind() {
		case "Deployment", "DaemonSet", "StatefulSet":
			if _, ok := vpaTargets[index]; !ok {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"VPA005",
					object.Identity(),
					false,
					"No VPA is found for object",
				))
			}
		}
	}

	return errors.LintRuleError{}
}
