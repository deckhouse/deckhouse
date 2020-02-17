package runner

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyContainerRules(object unstructured.Unstructured) error {
	var containers []v1.Container

	switch object.GetKind() {
	case "Deployment":
		newObject := appsv1.Deployment{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.Template.Spec.Containers
	case "DaemonSet":
		newObject := appsv1.DaemonSet{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.Template.Spec.Containers
	case "StatefulSet":
		newObject := appsv1.StatefulSet{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.Template.Spec.Containers
	case "Pod":
		newObject := v1.Pod{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.Containers

		containers = newObject.Spec.Containers
	case "Job":
		newObject := batchv1.Job{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.Template.Spec.Containers
	case "CronJob":
		newObject := batchv1beta1.CronJob{}
		data, _ := yaml.Marshal(object.Object)
		_ = yaml.Unmarshal(data, &newObject)

		containers = newObject.Spec.JobTemplate.Spec.Template.Spec.Containers
	}

	names := make(map[string]struct{})
	for _, container := range containers {
		if _, ok := names[container.Name]; ok == true {
			return fmt.Errorf("container %q already exists", container.Name)
		}
		names[container.Name] = struct{}{}

		if container.ImagePullPolicy != "" && container.ImagePullPolicy != "IfNotPresent" {
			return fmt.Errorf("container %q has imagePullPolicy option setted to not \"IfNotPresent\": %q", container.Name, container.ImagePullPolicy)
		}
		if !strings.HasPrefix(container.Image, "registry.flant.com") {
			return fmt.Errorf("container %q has image from an outer registry: %s", container.Name, container.Image)
		}

		envVariables := make(map[string]struct{})
		for _, variable := range container.Env {
			if _, ok := envVariables[variable.Name]; ok == true {
				return fmt.Errorf("container %q has two env variables with same name: %s", container.Name, variable.Name)
			}
			envVariables[variable.Name] = struct{}{}
		}
	}
	return nil
}

func applyObjectRules(object unstructured.Unstructured) error {
	// Check labels
	labels := object.GetLabels()
	if _, ok := labels["module"]; !ok {
		return fmt.Errorf("object %s/%s does not have label \"module\": %v", object.GetKind(), object.GetName(), labels)
	}
	if _, ok := labels["heritage"]; !ok {
		return fmt.Errorf("object %s/%s does not have label \"heritage\": %v", object.GetKind(), object.GetName(), labels)
	}

	// Check API versions
	switch object.GetKind() {
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		if object.GetAPIVersion() != "rbac.authorization.k8s.io/v1" {
			return fmt.Errorf("object %s/%s defined using deprecated api version %q, wanted \"rbac.authorization.k8s.io/v1\"", object.GetKind(), object.GetName(), object.GetAPIVersion())
		}
	case "Deployment", "DaemonSet", "StatefulSet":
		if object.GetAPIVersion() != "apps/v1" {
			return fmt.Errorf("object %s/%s defined using deprecated api version %q, wanted \"apps/v1\"", object.GetKind(), object.GetName(), object.GetAPIVersion())
		}
	}

	return nil
}

func ApplyLintRules(objectStore UnstructuredObjectStore) error {
	for _, object := range objectStore {
		err := applyObjectRules(object)
		if err != nil {
			return err
		}
		err = applyContainerRules(object)
		if err != nil {
			return err
		}
	}
	return nil
}
