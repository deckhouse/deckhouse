package rules

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/roles"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
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
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-ingress-nginx" {
		return true
	}
	// Istio kiali needs to patch index.html file
	if o.Unstructured.GetKind() == "Deployment" && o.Unstructured.GetNamespace() == "d8-istio" &&
		o.Unstructured.GetName() == "kiali" {
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

type ObjectLinter struct {
	ObjectStore    *storage.UnstructuredObjectStore
	ErrorsList     *errors.LintRuleErrorsList
	Module         utils.Module
	Values         string
	EnabledModules map[string]struct{}
}

func (l *ObjectLinter) CheckModuleEnabled(name string) bool {
	_, ok := l.EnabledModules[name]
	return ok
}

func (l *ObjectLinter) ApplyContainerRules(object storage.StoreObject) {
	containers, err := object.GetContainers()
	if err != nil {
		panic(err)
	}
	if len(containers) == 0 {
		return
	}

	l.ErrorsList.Add(containerNameDuplicates(object, containers))
	l.ErrorsList.Add(containerEnvVariablesDuplicates(object, containers))
	l.ErrorsList.Add(containerImageTagLatest(object, containers))
	l.ErrorsList.Add(containerImagePullPolicyIfNotPresent(object, containers))

	if !skipObjectIfNeeded(&object) {
		l.ErrorsList.Add(containerStorageEphemeral(object, containers))
		l.ErrorsList.Add(containerSecurityContext(object, containers))
		l.ErrorsList.Add(containerPorts(object, containers))
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

func (l *ObjectLinter) ApplyObjectRules(object storage.StoreObject) {
	l.ErrorsList.Add(objectRecommendedLabels(object))
	l.ErrorsList.Add(objectAPIVersion(object))
	if l.CheckModuleEnabled("priority-class") {
		l.ErrorsList.Add(objectPriorityClass(object))
	}
	l.ErrorsList.Add(objectDNSPolicy(object))

	l.ErrorsList.Add(roles.ObjectUserAuthzClusterRolePath(l.Module, object))
	l.ErrorsList.Add(roles.ObjectRBACPlacement(l.Module, object))
	l.ErrorsList.Add(roles.ObjectBindingSubjectServiceAccountCheck(l.Module, object, l.ObjectStore))

	if !skipObjectIfNeeded(&object) {
		l.ErrorsList.Add(objectSecurityContext(object))
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

func newConvertError(object storage.StoreObject, err error) errors.LintRuleError {
	return errors.NewLintRuleError(
		"MANIFEST007",
		object.Identity(),
		nil,
		"Cannot convert object to %s: %v", object.Unstructured.GetKind(), err,
	)
}

func objectPriorityClass(object storage.StoreObject) errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	converter := runtime.DefaultUnstructuredConverter

	var priorityClass string

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = deployment.Spec.Template.Spec.PriorityClassName
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = daemonset.Spec.Template.Spec.PriorityClassName
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return newConvertError(object, err)
		}

		priorityClass = statefulset.Spec.Template.Spec.PriorityClassName
	default:
		return errors.EmptyRuleError
	}

	switch priorityClass {
	case "":
		return errors.NewLintRuleError(
			"MANIFEST007",
			object.Identity(),
			priorityClass,
			"Priority class must not be empty",
		)
	case "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low" /* TODO: delete after migrating to 1.19 -> */, "cluster-critical":
	default:
		return errors.NewLintRuleError(
			"MANIFEST007",
			object.Identity(),
			priorityClass,
			"Priority class is not allowed",
		)
	}

	return errors.EmptyRuleError
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

func objectDNSPolicy(object storage.StoreObject) errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	name := object.Unstructured.GetName()
	converter := runtime.DefaultUnstructuredConverter

	var dnsPolicy string
	var hostNetwork bool

	switch kind {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(deployment.Spec.Template.Spec.DNSPolicy)
		hostNetwork = deployment.Spec.Template.Spec.HostNetwork
	case "DaemonSet":
		daemonset := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), daemonset)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(daemonset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = daemonset.Spec.Template.Spec.HostNetwork
	case "StatefulSet":
		statefulset := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), statefulset)
		if err != nil {
			return newConvertError(object, err)
		}

		dnsPolicy = string(statefulset.Spec.Template.Spec.DNSPolicy)
		hostNetwork = statefulset.Spec.Template.Spec.HostNetwork
	default:
		return errors.EmptyRuleError
	}

	switch name {
	case "cloud-controller-manager", "machine-controller-manager":
		if hostNetwork && dnsPolicy != "Default" {
			return errors.NewLintRuleError(
				"MANIFEST007",
				object.Identity(),
				dnsPolicy,
				"dnsPolicy must be `Default` with hostNetwork = `true`",
			)
		}
	case "deckhouse":
		if hostNetwork && (dnsPolicy != "Default" && dnsPolicy != "ClusterFirstWithHostNet") {
			return errors.NewLintRuleError(
				"MANIFEST007",
				object.Identity(),
				dnsPolicy,
				"dnsPolicy must be `Default` or `ClusterFirstWithHostNet` with hostNetwork = `true`",
			)
		}
	default:
		if hostNetwork && dnsPolicy != "ClusterFirstWithHostNet" {
			return errors.NewLintRuleError(
				"MANIFEST007",
				object.Identity(),
				dnsPolicy,
				"dnsPolicy must be `ClusterFirstWithHostNet` with hostNetwork = `true`",
			)
		}
	}
	return errors.EmptyRuleError
}
