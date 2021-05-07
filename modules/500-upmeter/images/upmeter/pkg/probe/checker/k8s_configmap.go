package checker

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/util"
)

// ConfigMapLifecycle is a checker constructor and configurator
type ConfigMapLifecycle struct {
	Access                   *kubernetes.Access
	Timeout                  time.Duration
	Namespace                string
	GarbageCollectionTimeout time.Duration
}

func (c ConfigMapLifecycle) Checker() check.Checker {
	return &configMapLifecycleChecker{
		access:                  c.Access,
		timeout:                 c.Timeout,
		namespace:               c.Namespace,
		garbageCollectorTimeout: c.GarbageCollectionTimeout,
	}
}

type configMapLifecycleChecker struct {
	access    *kubernetes.Access
	namespace string
	timeout   time.Duration

	garbageCollectorTimeout time.Duration

	// inner state
	checker check.Checker
}

func (c *configMapLifecycleChecker) BusyWith() string {
	return c.checker.BusyWith()
}

func (c *configMapLifecycleChecker) Check() check.Error {
	configMap := createConfigMap()
	c.checker = c.new(configMap)
	return c.checker.Check()
}

/*
 1. check control plane availability
 2. collect the garbage of the configmap from previous runs
 3. create and delete the configmap in api
 4. ensure it does not exist (with retries)
*/
func (c *configMapLifecycleChecker) new(configMap *v1.ConfigMap) check.Checker {
	verifyNotListed := withRetryEachSeconds(
		&objectIsNotListedChecker{
			access:    c.access,
			namespace: c.namespace,
			kind:      configMap.Kind,
			listOpts:  listOptsByName(configMap.Name),
		},
		c.timeout)

	collectGarbage := newGarbageCollectorCheckerByLabels(
		c.access,
		configMap.Kind,
		c.namespace,
		configMap.GetLabels(),
		c.garbageCollectorTimeout)

	create := &configMapCreationChecker{access: c.access, configMap: configMap, namespace: c.namespace}
	delete := &configMapDeletionChecker{access: c.access, configMap: configMap, namespace: c.namespace}

	check := sequence(
		&controlPlaneChecker{c.access},
		collectGarbage,
		create,
		delete,
		verifyNotListed,
	)

	return withTimeout(check, c.timeout)
}

type configMapCreationChecker struct {
	access    *kubernetes.Access
	namespace string
	configMap *v1.ConfigMap
}

func (c *configMapCreationChecker) BusyWith() string {
	return fmt.Sprintf("creating configmap %s/%s", c.namespace, c.configMap.Name)
}

func (c *configMapCreationChecker) Check() check.Error {
	client := c.access.Kubernetes()

	_, err := client.CoreV1().ConfigMaps(c.namespace).Create(c.configMap)
	if err != nil {
		return check.ErrUnknown("creating configMap %s/%s: %v", c.namespace, c.configMap.Name, err)
	}

	return nil
}

type configMapDeletionChecker struct {
	access    *kubernetes.Access
	configMap *v1.ConfigMap
	namespace string
}

func (c *configMapDeletionChecker) BusyWith() string {
	return fmt.Sprintf("deleting configmap %s/%s", c.namespace, c.configMap.Name)
}

func (c *configMapDeletionChecker) Check() check.Error {
	client := c.access.Kubernetes()

	err := client.CoreV1().ConfigMaps(c.namespace).Delete(c.configMap.Name, &metav1.DeleteOptions{})
	if err != nil {
		return check.ErrFail("deleting configMap %s/%s: %v", c.namespace, c.configMap.Name, err)
	}

	return nil
}

func createConfigMap() *v1.ConfigMap {
	name := util.RandomIdentifier("upmeter-basic")

	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				"upmeter-agent": util.AgentUniqueId(),
				"upmeter-group": "control-plane",
				"upmeter-probe": "basic",
			},
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
}
