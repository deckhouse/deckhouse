package rules

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/resources"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/roles"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

func skipObjectIfNeeded(o *storage.StoreObject) bool {
	// Dynatrace module deprecated and will be removed
	if o.Unstructured.GetKind() == "Deployment" && o.Unstructured.GetNamespace() == "d8-dynatrace" {
		return true
	}
	// Control plane configurator module used only in kops clusters and will be removed
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-system" &&
		o.Unstructured.GetName() == "control-plane-configurator" {
		return true
	}
	// Control plane proxy uses `flant/kube-ca-auth-proxy` with nginx and should be refactored
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-monitoring" &&
		strings.HasPrefix(o.Unstructured.GetName(), "control-plane-proxy") {
		return true
	}
	// Ingress Nginx has a lot of hardcoded configuration, which makes it hard to get secured
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-ingress-nginx" &&
		strings.HasPrefix(o.Unstructured.GetName(), "controller") {
		return true
	}

	return false
}

func skipObjectContainerIfNeeded(o *storage.StoreObject, c *v1.Container) bool {
	// Control plane manager image-holder containers run `/pause` and has no additional parameters
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "kube-system" &&
		o.Unstructured.GetName() == "d8-control-plane-manager" &&
		strings.HasPrefix(c.Name, "image-holder") {
		return true
	}
	// Coredns listens :53 port in hostNetwork
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-system" &&
		o.Unstructured.GetName() == "node-local-dns" && c.Name == "coredns" {
		return true
	}

	return false
}

func applyContainerRules(lintRuleErrorsList *errors.LintRuleErrorsList, object storage.StoreObject) {
	containers, err := object.GetContainers()
	if err != nil {
		panic(err)
	}
	if len(containers) == 0 {
		return
	}

	lintRuleErrorsList.Add(containerNameDuplicates(object, containers))
	lintRuleErrorsList.Add(containerEnvVariablesDuplicates(object, containers))
	lintRuleErrorsList.Add(containerImageTagLatest(object, containers))
	lintRuleErrorsList.Add(containerImagePullPolicyIfNotPresent(object, containers))

	if !skipObjectIfNeeded(&object) {
		lintRuleErrorsList.Add(containerStorageEphemeral(object, containers))
		lintRuleErrorsList.Add(containerSecurityContext(object, containers))
		lintRuleErrorsList.Add(containerPorts(object, containers))
	}
}

func containerNameDuplicates(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	names := make(map[string]struct{})
	for _, c := range containers {
		if _, ok := names[c.Name]; ok {
			return errors.NewLintRuleError("CONTAINER001", object.Identity(), c.Name, "Duplicate container name")
		}
		names[c.Name] = struct{}{}
	}
	return errors.EmptyRuleError
}

func containerEnvVariablesDuplicates(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		envVariables := make(map[string]struct{})
		for _, variable := range c.Env {
			if _, ok := envVariables[variable.Name]; ok {
				return errors.NewLintRuleError(
					"CONTAINER002",
					object.Identity()+"; container = "+c.Name,
					variable.Name,
					"Container has two env variables with same name",
				)
			}
			envVariables[variable.Name] = struct{}{}
		}
	}
	return errors.EmptyRuleError
}

func containerImageTagLatest(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		imageParts := strings.Split(c.Image, ":")
		if len(imageParts) != 2 {
			return errors.NewLintRuleError(
				"CONTAINER003",
				object.Identity()+"; container = "+c.Name,
				nil,
				"Can't parse an image for container",
			)
		}
		if imageParts[1] == "latest" {
			return errors.NewLintRuleError("CONTAINER003",
				object.Identity()+"; container = "+c.Name,
				nil,
				"Image tag \"latest\" used",
			)
		}
	}
	return errors.EmptyRuleError
}

func containerImagePullPolicyIfNotPresent(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		if c.ImagePullPolicy == "" || c.ImagePullPolicy == "IfNotPresent" {
			continue
		}
		return errors.NewLintRuleError(
			"CONTAINER004",
			object.Identity()+"; container = "+c.Name,
			c.ImagePullPolicy,
			"Container imagePullPolicy should be unspecified or \"IfNotPresent\"",
		)
	}
	return errors.EmptyRuleError
}

func containerStorageEphemeral(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		if skipObjectContainerIfNeeded(&object, &c) {
			continue
		}
		if c.Resources.Requests.StorageEphemeral() == nil || c.Resources.Requests.StorageEphemeral().Value() == 0 {
			return errors.NewLintRuleError(
				"CONTAINER006",
				object.Identity()+"; container = "+c.Name,
				nil,
				"Container StorageEphemeral is not defined in Resources.Requests",
			)
		}
	}
	return errors.EmptyRuleError
}

func containerSecurityContext(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		if skipObjectContainerIfNeeded(&object, &c) {
			continue
		}
		if c.SecurityContext == nil {
			return errors.NewLintRuleError(
				"CONTAINER005",
				object.Identity()+"; container = "+c.Name,
				nil,
				"Container SecurityContext is not defined",
			)
		}
	}
	return errors.EmptyRuleError
}

func containerPorts(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		if skipObjectContainerIfNeeded(&object, &c) {
			continue
		}
		for _, p := range c.Ports {
			if p.ContainerPort <= 1024 {
				return errors.NewLintRuleError(
					"CONTAINER006",
					object.Identity()+"; container = "+c.Name,
					p.ContainerPort,
					"Container uses port <= 1024",
				)
			}
		}
	}
	return errors.EmptyRuleError
}

func applyObjectRules(objectStore *storage.UnstructuredObjectStore, lintRuleErrorsList *errors.LintRuleErrorsList, module types.Module, object storage.StoreObject) {
	lintRuleErrorsList.Add(objectRecommendedLabels(object))
	lintRuleErrorsList.Add(objectAPIVersion(object))
	lintRuleErrorsList.Add(roles.ObjectUserAuthzClusterRolePath(module, object))
	lintRuleErrorsList.Add(roles.ObjectDeckhouseClusterRoles(module, object))
	lintRuleErrorsList.Add(roles.ObjectRBACPlacement(module, object))
	lintRuleErrorsList.Add(roles.ObjectBindingSubjectServiceAccountCheck(module, object, objectStore))

	if !skipObjectIfNeeded(&object) {
		lintRuleErrorsList.Add(objectSecurityContext(object))
	}
}

func objectRecommendedLabels(object storage.StoreObject) errors.LintRuleError {
	labels := object.Unstructured.GetLabels()
	if _, ok := labels["module"]; !ok {
		return errors.NewLintRuleError(
			"MANIFEST001",
			object.Identity(),
			labels,
			"Object does not have the label \"module\"",
		)
	}
	if _, ok := labels["heritage"]; !ok {
		return errors.NewLintRuleError(
			"MANIFEST001",
			object.Identity(),
			labels,
			"Object does not have the label \"heritage\"",
		)
	}
	return errors.EmptyRuleError
}

func newAPIVersionError(wanted, version, objectID string) errors.LintRuleError {
	if version != wanted {
		return errors.NewLintRuleError(
			"MANIFEST002",
			objectID,
			version,
			"Object defined using deprecated api version, wanted %q", wanted,
		)
	}
	return errors.EmptyRuleError
}

func objectAPIVersion(object storage.StoreObject) errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	version := object.Unstructured.GetAPIVersion()

	switch kind {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return newAPIVersionError("rbac.authorization.k8s.io/v1", version, object.Identity())
	case "Deployment", "DaemonSet", "StatefulSet":
		return newAPIVersionError("apps/v1", version, object.Identity())
	case "Ingress":
		return newAPIVersionError("networking.k8s.io/v1beta1", version, object.Identity())
	case "PriorityClass":
		return newAPIVersionError("scheduling.k8s.io/v1", version, object.Identity())
	case "PodSecurityPolicy":
		return newAPIVersionError("policy/v1beta1", version, object.Identity())
	case "NetworkPolicy":
		return newAPIVersionError("networking.k8s.io/v1", version, object.Identity())
	default:
		return errors.EmptyRuleError
	}
}

func objectSecurityContext(object storage.StoreObject) errors.LintRuleError {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return errors.EmptyRuleError
	}

	securityContext, err := object.GetPodSecurityContext()
	if err != nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			fmt.Sprintf("GetPodSecurityContext failed: %v", err),
		)
	}

	if securityContext == nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			"Object's SecurityContext is not defined",
		)
	}
	if securityContext.RunAsNonRoot == nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			"Object's SecurityContext missing parameter RunAsNonRoot",
		)
	}

	if securityContext.RunAsUser == nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			"Object's SecurityContext missing parameter RunAsUser",
		)
	}
	if securityContext.RunAsGroup == nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			"Object's SecurityContext missing parameter RunAsGroup",
		)
	}
	switch *securityContext.RunAsNonRoot {
	case true:
		if *securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534 {
			return errors.NewLintRuleError(
				"MANIFEST003",
				object.Identity(),
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534",
			)
		}
	case false:
		if *securityContext.RunAsUser != 0 || *securityContext.RunAsGroup != 0 {
			return errors.NewLintRuleError(
				"MANIFEST003",
				object.Identity(),
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: false`, but RunAsUser:RunAsGroup differs from 0:0",
			)
		}
	}

	return errors.EmptyRuleError
}

func ApplyLintRules(module types.Module, values string, objectStore *storage.UnstructuredObjectStore) error {
	var lintRuleErrorsList errors.LintRuleErrorsList
	for _, object := range objectStore.Storage {
		applyObjectRules(objectStore, &lintRuleErrorsList, module, object)
		applyContainerRules(&lintRuleErrorsList, object)
	}

	resources.ControllerMustHaveVPA(module, values, objectStore, &lintRuleErrorsList)
	resources.ControllerMustHavePDB(objectStore, &lintRuleErrorsList)

	return lintRuleErrorsList.ConvertToError()
}
