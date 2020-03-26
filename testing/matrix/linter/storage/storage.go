package storage

import (
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceIndex struct {
	Kind      string
	Name      string
	Namespace string
}

func (g *ResourceIndex) AsString() string {
	if g.Namespace == "" {
		return g.Kind + "/" + g.Name
	}

	return g.Namespace + "/" + g.Kind + "/" + g.Name
}

type StoreObject struct {
	Path         string
	Unstructured unstructured.Unstructured
}

func GetResourceIndex(object StoreObject) ResourceIndex {
	return ResourceIndex{
		Kind:      object.Unstructured.GetKind(),
		Name:      object.Unstructured.GetName(),
		Namespace: object.Unstructured.GetNamespace(),
	}
}

func (s *StoreObject) GetContainers() ([]v1.Container, error) {
	var containers []v1.Container
	converter := runtime.DefaultUnstructuredConverter

	switch s.Unstructured.GetKind() {
	case "Deployment":
		deployment := new(appsv1.Deployment)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), deployment)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Deployment failed: %v", err)
		}

		containers = deployment.Spec.Template.Spec.Containers
	case "DaemonSet":
		daemonSet := new(appsv1.DaemonSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), daemonSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to DaemonSet failed: %v", err)
		}

		containers = daemonSet.Spec.Template.Spec.Containers
	case "StatefulSet":
		statefulSet := new(appsv1.StatefulSet)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), statefulSet)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to StatefulSet failed: %v", err)
		}

		containers = statefulSet.Spec.Template.Spec.Containers
	case "Pod":
		pod := new(v1.Pod)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), pod)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Pod failed: %v", err)
		}

		containers = pod.Spec.Containers
	case "Job":
		job := new(batchv1.Job)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), job)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to Job failed: %v", err)
		}

		containers = job.Spec.Template.Spec.Containers
	case "CronJob":
		cronJob := new(batchv1beta1.CronJob)

		err := converter.FromUnstructured(s.Unstructured.UnstructuredContent(), cronJob)
		if err != nil {
			return []v1.Container{}, fmt.Errorf("convert Unstructured to CronJob failed: %v", err)
		}

		containers = cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
	}
	return containers, nil
}

func (s *StoreObject) ShortPath() string {
	return strings.Join(strings.Split(s.Path, string(os.PathSeparator))[1:], string(os.PathSeparator))
}

func (s *StoreObject) Identity() string {
	kind := s.Unstructured.GetKind()
	name := s.Unstructured.GetName()
	namespace := s.Unstructured.GetNamespace()

	if namespace == "" {
		return fmt.Sprintf("kind = %s ; name = %s", kind, name)
	}
	return fmt.Sprintf("kind = %s ; name =  %s ; namespace = %s", kind, name, namespace)
}

type UnstructuredObjectStore struct {
	Storage map[ResourceIndex]StoreObject
}

func NewUnstructuredObjectStore() UnstructuredObjectStore {
	return UnstructuredObjectStore{Storage: make(map[ResourceIndex]StoreObject)}
}

// Put object into unstructured store
func (s *UnstructuredObjectStore) Put(path string, object map[string]interface{}) error {
	var u unstructured.Unstructured
	u.SetUnstructuredContent(object)

	storeObject := StoreObject{Path: path, Unstructured: u}

	index := GetResourceIndex(storeObject)
	if _, ok := s.Storage[index]; ok {
		return fmt.Errorf("object %q already exists in the object store", index.AsString())
	}

	s.Storage[index] = storeObject
	return nil
}

// Get object from unstructured store
func (s *UnstructuredObjectStore) Get(key ResourceIndex) StoreObject {
	return s.Storage[key]
}

func (s *UnstructuredObjectStore) Exists(key ResourceIndex) bool {
	_, ok := s.Storage[key]
	return ok
}

func (s *UnstructuredObjectStore) Close() {
	s.Storage = nil
}
