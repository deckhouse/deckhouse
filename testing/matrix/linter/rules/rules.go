/*
Copyright 2021 Flant JSC

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

package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/modules"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/roles"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

const defaultRegistry = "registry.example.com/deckhouse"

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
	if o.Unstructured.GetKind() == "DaemonSet" && (o.Unstructured.GetNamespace() == "d8-system") || (o.Unstructured.GetNamespace() == "kube-system") &&
		o.Unstructured.GetName() == "node-local-dns" && c.Name == "coredns" {
		return true
	}
	// Chrony listens :123 port in hostNetwork
	if o.Unstructured.GetKind() == "DaemonSet" && o.Unstructured.GetNamespace() == "d8-chrony" &&
		strings.HasPrefix(o.Unstructured.GetName(), "chrony") && c.Name == "chrony" {
		return true
	}

	return false
}

func skipObjectWithWildCardIfNeeded(o *storage.StoreObject) bool {
	// skip file with object `d8:admission-policy-engine:gatekeeper`
	if o.Path == "admission-policy-engine/templates/rbac-for-us.yaml" &&
		o.Unstructured.GetName() == "d8:admission-policy-engine:gatekeeper" {
		return true
	}

	return false
}

type ObjectLinter struct {
	ObjectStore    *storage.UnstructuredObjectStore
	ErrorsList     *errors.LintRuleErrorsList
	Module         utils.Module
	EnabledModules set.Set
}

func (l *ObjectLinter) ApplyContainerRules(object storage.StoreObject) {
	containers, err := object.GetContainers()
	if err != nil {
		panic(err)
	}
	initContainers, err := object.GetInitContainers()
	if err != nil {
		panic(err)
	}
	containers = append(initContainers, containers...)
	if len(containers) == 0 {
		return
	}

	l.ErrorsList.Add(containerNameDuplicates(object, containers))
	l.ErrorsList.Add(containerEnvVariablesDuplicates(object, containers))
	l.ErrorsList.Add(containerImageDigestCheck(object, containers))
	l.ErrorsList.Add(containersImagePullPolicy(object, containers))

	if !skipObjectIfNeeded(&object) {
		l.ErrorsList.Add(containerStorageEphemeral(object, containers))
		l.ErrorsList.Add(containerSecurityContext(object, containers))
		l.ErrorsList.Add(containerPorts(object, containers))
	}
}

func containersImagePullPolicy(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	o := object.Unstructured
	if o.GetNamespace() == "d8-system" && o.GetKind() == "Deployment" && o.GetName() == "deckhouse" {
		c := containers[0]
		if c.ImagePullPolicy != "Always" {
			// image pull policy must be Always,
			// because changing d8-system/deckhouse-registry triggers restart deckhouse deployment
			// d8-system/deckhouse-registry can contain invalid registry creds
			// and restarting deckhouse with invalid creads will break all static pods on masters
			// and bashible
			return errors.NewLintRuleError(
				"CONTAINER004",
				object.Identity()+"; container = "+c.Name,
				c.ImagePullPolicy,
				"Container imagePullPolicy should be unspecified or \"Always\"",
			)
		}

		return errors.EmptyRuleError
	}

	return containerImagePullPolicyIfNotPresent(object, containers)
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

func shouldSkipModuleContainer(module string, container string) bool {
	// okmeter module uses images from external repo - registry.okmeter.io/agent/okagent:stub
	if module == "okmeter" && container == "okagent" {
		return true
	}
	// control-plane-manager uses `$images` as dict to render static pod manifests,
	// so we cannot use helm lib `helm_lib_module_image` helper because `$images`
	// is also rendered in `dhctl` tool on cluster bootstrap.
	if module == "d8-control-plane-manager" && strings.HasPrefix(container, "image-holder") {
		return true
	}
	return false
}

func containerImageDigestCheck(object storage.StoreObject, containers []v1.Container) errors.LintRuleError {
	for _, c := range containers {
		if shouldSkipModuleContainer(object.Unstructured.GetName(), c.Name) {
			continue
		}

		re := regexp.MustCompile(`(?P<repository>.+)(@|:)imageHash[-a-z0-9A-Z]+$`)
		match := re.FindStringSubmatch(c.Image)
		repo, err := name.NewRepository(match[re.SubexpIndex("repository")])
		if err != nil {
			return errors.NewLintRuleError("CONTAINER003",
				object.Identity()+"; container = "+c.Name,
				nil,
				"Cannot parse repository from image: "+c.Image,
			)
		}

		if repo.Name() != defaultRegistry {
			return errors.NewLintRuleError("CONTAINER003",
				object.Identity()+"; container = "+c.Name,
				nil,
				"All images must be deployed from the same default registry: "+defaultRegistry+" current:"+repo.RepositoryStr(),
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
				"Ephemeral storage for container is not defined in Resources.Requests",
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
	l.ErrorsList.Add(namespaceLabels(object))
	l.ErrorsList.Add(objectAPIVersion(object))
	if l.EnabledModules.Has("priority-class") {
		l.ErrorsList.Add(objectPriorityClass(object))
	}
	l.ErrorsList.Add(objectDNSPolicy(object))

	l.ErrorsList.Add(roles.ObjectUserAuthzClusterRolePath(l.Module, object))
	l.ErrorsList.Add(roles.ObjectRBACPlacement(l.Module, object))
	l.ErrorsList.Add(roles.ObjectBindingSubjectServiceAccountCheck(l.Module, object, l.ObjectStore))

	if !skipObjectIfNeeded(&object) {
		l.ErrorsList.Add(objectSecurityContext(object))
	}

	l.ErrorsList.Add(objectRevisionHistoryLimit(object))
	l.ErrorsList.Add(objectHostNetworkPorts(object))

	l.ErrorsList.Add(modules.PromtoolRuleCheck(l.Module, object))

	if !skipObjectWithWildCardIfNeeded(&object) {
		l.ErrorsList.Add(roles.ObjectRolesWildcard(object))
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

func namespaceLabels(object storage.StoreObject) errors.LintRuleError {
	if object.Unstructured.GetKind() != "Namespace" {
		return errors.EmptyRuleError
	}

	if !strings.HasPrefix(object.Unstructured.GetName(), "d8-") {
		return errors.EmptyRuleError
	}

	labels := object.Unstructured.GetLabels()

	if label := labels["prometheus.deckhouse.io/rules-watcher-enabled"]; label == "true" {
		return errors.EmptyRuleError
	}

	return errors.NewLintRuleError(
		"MANIFEST001",
		object.Identity(),
		labels,
		"Namespace object does not have the label \"prometheus.deckhouse.io/rules-watcher-enabled\"")
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
		return newAPIVersionError("networking.k8s.io/v1", version, object.Identity())
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

func objectRevisionHistoryLimit(object storage.StoreObject) errors.LintRuleError {
	if object.Unstructured.GetKind() == "Deployment" {
		converter := runtime.DefaultUnstructuredConverter
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return newConvertError(object, err)
		}

		// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#revision-history-limit
		// Revision history limit controls the number of replicasets stored in the cluster for each deployment.
		// Higher number means higher resource consumption, lower means inability to rollback.
		//
		// Since Deckhouse does not use rollback, we can set it to 2 to be able to manually check the previous version.
		// It is more important to reduce the control plane pressure.
		maxHistoryLimit := int32(2)
		actualLimit := deployment.Spec.RevisionHistoryLimit

		if actualLimit == nil {
			return errors.NewLintRuleError(
				"MANIFEST008",
				object.Identity(),
				nil,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		}

		if *actualLimit > maxHistoryLimit {
			return errors.NewLintRuleError(
				"MANIFEST008",
				object.Identity(),
				*actualLimit,
				"Deployment spec.revisionHistoryLimit must be less or equal to %d", maxHistoryLimit,
			)
		}
	}
	return errors.EmptyRuleError
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
		if (*securityContext.RunAsUser != 65534 || *securityContext.RunAsGroup != 65534) &&
			(*securityContext.RunAsUser != 64535 || *securityContext.RunAsGroup != 64535) {
			return errors.NewLintRuleError(
				"MANIFEST003",
				object.Identity(),
				fmt.Sprintf("%d:%d", *securityContext.RunAsUser, *securityContext.RunAsGroup),
				"Object's SecurityContext has `RunAsNonRoot: true`, but RunAsUser:RunAsGroup differs from 65534:65534 (nobody) or 64535:64535 (deckhouse)",
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

func objectHostNetworkPorts(object storage.StoreObject) errors.LintRuleError {
	switch object.Unstructured.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Pod", "Job", "CronJob":
	default:
		return errors.EmptyRuleError
	}

	hostNetworkUsed, err := object.IsHostNetwork()
	if err != nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			fmt.Sprintf("IsHostNetwork failed: %v", err),
		)
	}
	if !hostNetworkUsed {
		return errors.EmptyRuleError
	}

	containers, err := object.GetContainers()
	if err != nil {
		return errors.NewLintRuleError(
			"MANIFEST003",
			object.Identity(),
			nil,
			fmt.Sprintf("GetContainers failed: %v", err),
		)
	}

	for _, c := range containers {
		for _, p := range c.Ports {
			if hostNetworkUsed && p.ContainerPort >= 10500 {
				return errors.NewLintRuleError(
					"CONTAINER007",
					object.Identity()+"; container = "+c.Name,
					p.ContainerPort,
					"Pod running in hostNetwork and it's container uses port >= 10500",
				)
			}
			if p.HostPort >= 10500 {
				return errors.NewLintRuleError(
					"CONTAINER007",
					object.Identity()+"; container = "+c.Name,
					p.HostPort,
					"Container uses hostPort >= 10500",
				)
			}
		}
	}

	return errors.EmptyRuleError
}

func objectDNSPolicy(object storage.StoreObject) errors.LintRuleError {
	kind := object.Unstructured.GetKind()
	name := object.Unstructured.GetName()
	namespace := object.Unstructured.GetNamespace()
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

	if shouldSkipDNSPolicyResource(name, kind, namespace, hostNetwork, dnsPolicy) {
		return errors.EmptyRuleError
	}

	if !hostNetwork {
		return errors.EmptyRuleError
	}

	if dnsPolicy == "ClusterFirstWithHostNet" {
		return errors.EmptyRuleError
	}

	return errors.NewLintRuleError(
		"MANIFEST007",
		object.Identity(),
		dnsPolicy,
		"dnsPolicy must be `ClusterFirstWithHostNet` when hostNetwork is `true`",
	)
}

func shouldSkipDNSPolicyResource(name string, kind string, namespace string, hostNetwork bool, dnsPolicy string) bool {
	switch name {
	// Cloud controller manager should work if cluster dns isn't responding or if cni isn't working
	case "cloud-controller-manager":
		return kind == "Deployment" && strings.HasPrefix(namespace, "d8-cloud-provider-") && hostNetwork && dnsPolicy == "Default"

	// Bashible-apiserver should work if cluster dns isn't responding or if cni isn't working
	case "bashible-apiserver":
		return kind == "Deployment" && namespace == "d8-cloud-instance-manager" && hostNetwork && dnsPolicy == "Default"

	// Deckhouse main pod use Default policy when cluster isn't bootstrapped
	case "deckhouse":
		return kind == "Deployment" && namespace == "d8-system" && hostNetwork && dnsPolicy == "Default"

	default:
		return false
	}
}
