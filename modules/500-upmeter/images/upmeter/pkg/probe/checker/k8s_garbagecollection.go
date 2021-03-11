package checker

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/check"
	k8s "upmeter/pkg/kubernetes"
)

// garbageCollectorChecker ensures objects can be listed and their deletion is complete
type garbageCollectorChecker struct {
	access    *k8s.Access
	namespace string
	kind      string
	listOpts  *metav1.ListOptions
	timeout   time.Duration

	// inner state
	firstRun bool
}

func newGarbageCollectorCheckerByLabels(access *k8s.Access, kind, namespace string, labels map[string]string, timeout time.Duration) check.Checker {
	return &garbageCollectorChecker{
		access:    access,
		namespace: namespace,
		kind:      kind,
		listOpts:  listOptsByLabels(labels),
		timeout:   timeout,

		// inner state
		firstRun: true,
	}
}

func (c *garbageCollectorChecker) BusyWith() string {
	return fmt.Sprintf("collecting garbage of %s/%s, listOpts=%s", c.namespace, c.kind, c.listOpts)
}

func (c *garbageCollectorChecker) Check() check.Error {
	defer func() {
		c.firstRun = false
	}()

	var err error
	var list []string

	client := c.access.Kubernetes()

	list, err = listObjects(client, c.kind, c.namespace, *c.listOpts)
	if err != nil {
		return check.ErrUnknown("listing %s/%s: %v", c.namespace, c.kind, err)
	}
	if len(list) == 0 {
		return nil
	}

	err = deleteObjects(client, c.kind, list)
	if err != nil {
		return check.ErrUnknown("cannot clean garbage %s/%s: %v", c.namespace, c.kind, err)
	}

	if c.firstRun {
		// Garbage was found on first run. Immediate Unknown result. But... why?
		return check.ErrUnknown("garbage found for %s/%s", c.namespace, c.kind)
	}

	// Wait until deletion
	count := int(c.timeout.Seconds())
	listErrors := 0
	for ; count > 0; count-- {
		// Sleep first to give time for API server to delete objects
		time.Sleep(time.Second)

		list, err = listObjects(client, c.kind, c.namespace, *c.listOpts)
		if err != nil {
			listErrors++
		} else if len(list) == 0 {
			return nil
		}
	}

	if err != nil {
		return check.ErrUnknown("garbage still present in %ss (names=%s): %v", c.kind, dumpNames(list), err)
	}
	return check.ErrUnknown("garbage still present in %ss (names=%s)", c.kind, dumpNames(list))

}
