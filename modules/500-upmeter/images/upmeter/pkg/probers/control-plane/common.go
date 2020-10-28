package control_plane

import (
	"fmt"
	"strings"
	"time"

	"github.com/flant/shell-operator/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/app"
	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/util"
)

const checkApiTimeout = 5 * time.Second

func CheckApiAvailable(pr *types.CommonProbe) bool {
	avail := true
	log := pr.LogEntry()
	// Check API server is available
	// Set Unknown result on timeout or on error.
	// Stop probe execution on error.
	util.DoWithTimer(checkApiTimeout, func() {
		_, err := pr.KubernetesClient.Discovery().ServerVersion()
		if err != nil {
			log.Errorf("Check API availability: %v", err)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
			avail = false
		}
	}, func() {
		log.Infof("Exceeds timeout '%s' when fetch /version", checkApiTimeout.String())
		pr.ResultCh <- pr.Result(types.ProbeUnknown)
	})
	return avail
}

const deleteGarbageTimeout = 10 * time.Second

// GarbageCollect list objects by labels, delete them and wait until deletion is complete.
// return 1 if deletion was successful or no objects found
// return 0 if list of delete operations give error or there are objects after deletion.
func GarbageCollect(pr *types.CommonProbe, kind string, labels map[string]string) bool {
	log := pr.LogEntry()

	var collected bool

	listOpts := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: labels,
		}),
	}

	// List and delete garbage objects.
	// Set Unknown result on timeout or on error if it is a first run.
	// Set Failed result on timeout or on error if not first run.
	// Stop probe execution on error.
	// Delete each Pod in list, wait while all Pods are gone.
	// Set Unknown result on deletion errors.
	// Set Unknown result on list errors on first run if Pods aren't gone.
	// Set Failed result on list errors after first run if Pods aren't gone.

	util.DoWithTimer(deleteGarbageTimeout, func() {
		var err error
		var list []string

		list, err = ListObjects(pr, kind, listOpts)
		if err != nil {
			log.Errorf("List %s: %v", kind, err)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
			return
		}
		if len(list) == 0 {
			// Success!!!
			collected = true
			return
		}
		log.Infof("Garbage %s detected: %s", kind, DumpNames(list))

		// Immediate Unknown result if garbage is found on first run.
		if pr.State().FirstRun {
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		}

		err = DeleteObjects(pr, kind, list)
		if err != nil {
			log.Errorf("Delete garbage %s: %v", kind, err)
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
			return
		}

		// Wait until deletion
		listErrors := 0
		count := int(deleteGarbageTimeout.Seconds())
		for i := 0; i < count; i++ {
			// Sleep first to give time for API server to delete objects
			time.Sleep(time.Second)
			list, err = ListObjects(pr, kind, listOpts)
			if err != nil {
				listErrors++
			} else {
				if len(list) == 0 {
					// Success!!!
					collected = true
					return
				}
			}
		}
		if listErrors > 0 {
			if err != nil {
				log.Errorf("List garbage %s: %s", kind, err)
			}
			log.Errorf("Stop probe. Garbage %s list is not empty: %s", kind, DumpNames(list))
			pr.ResultCh <- pr.Result(types.ProbeUnknown)
		}

		// Garbage is not collected.
	}, func() {
		log.Infof("Exceed timeout when listing garbage %s", kind)
		pr.ResultCh <- pr.Result(types.ProbeUnknown)
	})

	return collected
}

func ListObjects(pr *types.CommonProbe, kind string, listOpts metav1.ListOptions) ([]string, error) {
	fn, ok := listFns[strings.ToLower(kind)]
	if !ok {
		return nil, fmt.Errorf("Possible bug!!! No list function for kind='%s'")
	}
	return fn(pr.KubernetesClient, listOpts)
}

func DeleteObjects(pr *types.CommonProbe, kind string, names []string) error {
	fn, ok := delFns[strings.ToLower(kind)]
	if !ok {
		return fmt.Errorf("Possible bug!!! No delete function for kind='%s'")
	}
	for _, name := range names {
		err := fn(pr.KubernetesClient, name)
		if err != nil {
			return err
		}
	}
	return nil
}

var listFns = map[string]func(client kube.KubernetesClient, listOpts metav1.ListOptions) ([]string, error){
	"namespace":  ListNamespaceNames,
	"configmap":  ListConfigMapNames,
	"pod":        ListPodNames,
	"deployment": ListDeployNames,
}

func ListNamespaceNames(client kube.KubernetesClient, listOpts metav1.ListOptions) ([]string, error) {
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

func ListConfigMapNames(client kube.KubernetesClient, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.CoreV1().ConfigMaps(app.Namespace).List(listOpts)
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

func ListPodNames(client kube.KubernetesClient, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.CoreV1().Pods(app.Namespace).List(listOpts)
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

func ListDeployNames(client kube.KubernetesClient, listOpts metav1.ListOptions) ([]string, error) {
	list, err := client.AppsV1().Deployments(app.Namespace).List(listOpts)
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

func DumpNames(list []string) string {
	if list == nil || len(list) == 0 {
		return ""
	}
	return strings.Join(list, ", ")
}

var delFns = map[string]func(client kube.KubernetesClient, name string) error{
	"namespace": func(client kube.KubernetesClient, name string) error {
		return client.CoreV1().Namespaces().Delete(name, &metav1.DeleteOptions{})
	},
	"configmap": func(client kube.KubernetesClient, name string) error {
		return client.CoreV1().ConfigMaps(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"pod": func(client kube.KubernetesClient, name string) error {
		return client.CoreV1().Pods(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
	"deployment": func(client kube.KubernetesClient, name string) error {
		return client.AppsV1().Deployments(app.Namespace).Delete(name, &metav1.DeleteOptions{})
	},
}

func WaitForObjectDeletion(pr *types.CommonProbe, timeout time.Duration, kind, name string) bool {
	listOpts := metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	}

	var listErrors = 0
	var listErr error
	var list []string
	var triesCount = int(timeout.Seconds())

	for i := 0; i < triesCount; i++ {
		// Sleep first to give time for API server to delete ConfigMap
		time.Sleep(time.Second)

		list, listErr = ListObjects(pr, kind, listOpts)
		if listErr != nil {
			listErrors++
		} else {
			if len(list) == 0 {
				return true
			}
		}
	}
	if listErrors > 0 {
		pr.LogEntry().Errorf("Error waiting for deletion %s/%s: %v", kind, name, listErr)
	}
	pr.LogEntry().Errorf("%s/%s is not gone during timeout of %s.", kind, name, timeout.String())
	return false
}
