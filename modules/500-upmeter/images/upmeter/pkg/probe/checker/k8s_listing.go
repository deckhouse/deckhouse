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

package checker

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"d8.io/upmeter/pkg/check"
	k8s "d8.io/upmeter/pkg/kubernetes"
)

// objectIsNotListedChecker ensures object is not in the list anymore
type objectIsNotListedChecker struct {
	access    k8s.Access
	namespace string
	kind      string
	listOpts  *metav1.ListOptions
}

func (c *objectIsNotListedChecker) Check() check.Error {
	list, err := listObjects(c.access.Kubernetes(), c.kind, c.namespace, *c.listOpts)
	if err != nil {
		return check.ErrFail("cannot list %s/%s %s", c.namespace, c.kind, c.listOpts)
	}
	if len(list) > 0 {
		return check.ErrFail("object %s/%s %s not deleted yet", c.namespace, c.kind, c.listOpts)
	}
	return nil
}

func listOptsByName(name string) *metav1.ListOptions {
	return &metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	}
}

func listOptsByLabels(lbl map[string]string) *metav1.ListOptions {
	return &metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: lbl,
		}),
	}
}

func listObjects(client kubernetes.Interface, kind, namespace string, listOpts metav1.ListOptions) ([]string, error) {
	fn, ok := listFns[strings.ToLower(kind)]
	if !ok {
		return nil, fmt.Errorf("no list function for kind=%q, must be coding error", kind)
	}
	return fn(client, namespace, listOpts)
}

func deleteObjects(client kubernetes.Interface, kind, namespace string, names []string) error {
	fn, ok := delFns[strings.ToLower(kind)]
	if !ok {
		return fmt.Errorf("no delete function for kind=%q, must be coding error", kind)
	}
	for _, name := range names {
		err := fn(client, namespace, name)
		if err != nil {
			return err
		}
	}
	return nil
}

var listFns = map[string]func(client kubernetes.Interface, namespace string, listOpts metav1.ListOptions) ([]string, error){
	"namespace":  listNamespaceNames,
	"configmap":  listConfigMapNames,
	"pod":        listPodNames,
	"deployment": listDeployNames,
}

func listNamespaceNames(client kubernetes.Interface, _ string, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.CoreV1().Namespaces().List(listOpts)
	if err != nil {
		return nil, err
	}
	if list == nil || len(list.Items) == 0 {
		return []string{}, nil
	}
	res := make([]string, 0)
	for _, obj := range list.Items {
		res = append(res, obj.Name)
	}
	return res, nil
}

func listConfigMapNames(client kubernetes.Interface, namespace string, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.CoreV1().ConfigMaps(namespace).List(listOpts)
	if err != nil {
		return nil, err
	}
	if list == nil || len(list.Items) == 0 {
		return []string{}, nil
	}
	res := make([]string, 0)
	for _, obj := range list.Items {
		res = append(res, obj.Name)
	}
	return res, nil
}

func listPodNames(client kubernetes.Interface, namespace string, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.CoreV1().Pods(namespace).List(listOpts)
	if err != nil {
		return nil, err
	}
	if list == nil || len(list.Items) == 0 {
		return []string{}, nil
	}
	res := make([]string, 0)
	for _, obj := range list.Items {
		res = append(res, obj.Name)
	}
	return res, nil
}

func listDeployNames(client kubernetes.Interface, namespace string, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.AppsV1().Deployments(namespace).List(listOpts)
	if err != nil {
		return nil, err
	}
	if list == nil || len(list.Items) == 0 {
		return []string{}, nil
	}
	res := make([]string, 0)
	for _, obj := range list.Items {
		res = append(res, obj.Name)
	}
	return res, nil
}

func dumpNames(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return strings.Join(list, ", ")
}

var delFns = map[string]func(client kubernetes.Interface, namespace, name string) error{
	"namespace": func(client kubernetes.Interface, _, name string) error {
		return client.CoreV1().Namespaces().Delete(name, &metav1.DeleteOptions{})
	},
	"configmap": func(client kubernetes.Interface, namespace, name string) error {
		return client.CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"pod": func(client kubernetes.Interface, namespace, name string) error {
		return client.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"deployment": func(client kubernetes.Interface, namespace, name string) error {
		return client.AppsV1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{})
	},
}
