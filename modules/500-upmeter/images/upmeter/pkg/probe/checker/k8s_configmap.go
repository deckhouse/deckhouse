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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/util"
)

// ConfigMapLifecycle is a checker constructor and configurator
type ConfigMapLifecycle struct {
	Access                    kubernetes.Access
	Timeout                   time.Duration
	Namespace                 string
	GarbageCollectionTimeout  time.Duration
	ControlPlaneAccessTimeout time.Duration
}

func (c ConfigMapLifecycle) Checker() check.Checker {
	return &configMapLifecycleChecker{
		access:                    c.Access,
		timeout:                   c.Timeout,
		namespace:                 c.Namespace,
		garbageCollectorTimeout:   c.GarbageCollectionTimeout,
		controlPlaneAccessTimeout: c.ControlPlaneAccessTimeout,
	}
}

type configMapLifecycleChecker struct {
	access    kubernetes.Access
	namespace string
	timeout   time.Duration

	garbageCollectorTimeout   time.Duration
	controlPlaneAccessTimeout time.Duration

	// inner state
	checker check.Checker
}

func (c *configMapLifecycleChecker) Check() check.Error {
	configMap := createConfigMapObject()
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
	pingControlPlane := newControlPlaneChecker(c.access, c.controlPlaneAccessTimeout)

	collectGarbage := newGarbageCollectorCheckerByName(
		c.access,
		configMap.Kind,
		c.namespace,
		configMap.GetName(),
		c.garbageCollectorTimeout)

	createAndDeleteConfigMap := withTimeout(
		sequence(
			// create
			&configMapCreationChecker{access: c.access, configMap: configMap, namespace: c.namespace},
			// delete
			&configMapDeletionChecker{access: c.access, configMap: configMap, namespace: c.namespace},
		),
		// common timeout
		c.timeout,
	)

	verifyNotListed := withRetryEachSeconds(
		&objectIsNotListedChecker{
			access:    c.access,
			namespace: c.namespace,
			kind:      configMap.Kind,
			listOpts:  listOptsByName(configMap.Name),
		},
		c.garbageCollectorTimeout)

	return sequence(
		pingControlPlane,
		collectGarbage,
		createAndDeleteConfigMap,
		verifyNotListed,
	)
}

type configMapCreationChecker struct {
	access    kubernetes.Access
	namespace string
	configMap *v1.ConfigMap
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
	access    kubernetes.Access
	configMap *v1.ConfigMap
	namespace string
}

func (c *configMapDeletionChecker) Check() check.Error {
	client := c.access.Kubernetes()

	err := client.CoreV1().ConfigMaps(c.namespace).Delete(c.configMap.Name, &metav1.DeleteOptions{})
	if err != nil {
		return check.ErrFail("deleting configMap %s/%s: %v", c.namespace, c.configMap.Name, err)
	}

	return nil
}

func createConfigMapObject() *v1.ConfigMap {
	name := util.RandomIdentifier("upmeter-basic")

	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":           "upmeter",
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
