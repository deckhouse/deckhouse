package resources

import (
	"fmt"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

var exclusions = map[string]func(r *storage.ResourceIndex) bool{
	// Controllers VPA is configured through cr settings
	"ingress-nginx": func(r *storage.ResourceIndex) bool {
		if r.Kind == "DaemonSet" && r.Namespace == "d8-ingress-nginx" && strings.HasPrefix(r.Name, "controller-") {
			return true
		}
		return false
	},
}

func checkVPAEnabled(values string) bool {
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

func ControllerMustHasVPA(m types.Module, values string, objectStore *storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList) {

	exceptionFunc := exclusions[m.Name]
	if exceptionFunc == nil {
		exceptionFunc = func(r *storage.ResourceIndex) bool { return false }
	}

	if !checkVPAEnabled(values) {
		return
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
				continue
			}

			containers, err := object.GetContainers()
			if err != nil {
				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"VPA005",
					object.Identity(),
					false,
					"Get containers list for the object failed: %v",
					err,
				))
				continue
			}
			for _, container := range containers {
				res := container.Resources.Requests
				if res.Cpu().IsZero() && res.Memory().IsZero() {
					continue
				}

				lintRuleErrorsList.Add(errors.NewLintRuleError(
					"VPA005",
					object.Identity()+"; container = "+container.Name,
					fmt.Sprintf("cpu = %s, memory = %s", res.Cpu().String(), res.Memory().String()),
					"The container must not have resources requests, because resources are managed by VPA",
				))
			}
		}
	}
}
