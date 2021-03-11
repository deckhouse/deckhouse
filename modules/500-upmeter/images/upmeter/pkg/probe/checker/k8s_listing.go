package checker

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"upmeter/pkg/app"

	"upmeter/pkg/check"
	k8s "upmeter/pkg/kubernetes"
)

// objectIsNotListedChecker ensures object is not in the list anymore
type objectIsNotListedChecker struct {
	access    *k8s.Access
	namespace string
	kind      string
	listOpts  *metav1.ListOptions
}

func (c *objectIsNotListedChecker) BusyWith() string {
	return fmt.Sprintf("tracking object deletion %s/%s %s", c.namespace, c.kind, c.listOpts)
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
		return nil, fmt.Errorf("Possible bug!!! No list function for kind='%s'", kind)
	}
	return fn(client, namespace, listOpts)
}

func deleteObjects(client kubernetes.Interface, kind string, names []string) error {
	fn, ok := delFns[strings.ToLower(kind)]
	if !ok {
		return fmt.Errorf("Possible bug!!! No delete function for kind='%s'", kind)
	}
	for _, name := range names {
		err := fn(client, name)
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

var delFns = map[string]func(client kubernetes.Interface, name string) error{
	"namespace": func(client kubernetes.Interface, name string) error {
		return client.CoreV1().Namespaces().Delete(name, &metav1.DeleteOptions{})
	},
	"configmap": func(client kubernetes.Interface, name string) error {
		return client.CoreV1().ConfigMaps(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"pod": func(client kubernetes.Interface, name string) error {
		return client.CoreV1().Pods(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"deployment": func(client kubernetes.Interface, name string) error {
		return client.AppsV1().Deployments(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
}
