// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type resourceReadinessChecker struct {
	kubeCl   *client.KubernetesClient
	resource *template.Resource

	logger log.Logger

	attempt int
}

// kind should be in lower case!
var kindsToAttempts = map[string]int{
	// in some cases deckhouse controller doesn't have time for set status field, and we have Ready nodegroup whe it is not Ready
	"nodegroup": 5,
}

func (c *resourceReadinessChecker) IsReady() (bool, error) {
	defer func() {
		c.attempt++
		c.logger.LogInfoF("\n")
	}()

	name := resourceName(c.resource)
	c.logger.LogDebugF("Resource %s readiness attempts: %d\n", name, c.attempt)

	logNotReadyYet := func() {
		c.logger.LogInfoF("Resource %s has not been ready yet\n", name)
	}

	c.logger.LogInfoF("Checking if resource %s is ready...\n", name)

	expectedAttempts := 1
	kind := strings.ToLower(c.resource.GVK.Kind)
	if attempts, ok := kindsToAttempts[kind]; ok {
		log.DebugF("Found custom attempts %d for kind\n", attempts, c.resource.GVK.Kind)
		expectedAttempts = attempts
	}

	// wait some attempts for set statuses in the resources
	if c.attempt < expectedAttempts {
		logNotReadyYet()
		c.logger.LogDebugF("Skip resource % readiness checking for waiting set status\n", name)
		return false, nil
	}

	gvr, doc, err := resourceToGVR(c.kubeCl, c.resource)
	if err != nil {
		logNotReadyYet()
		c.logger.LogDebugF("Resource %s to GVR failed: %s\n", name, err)
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
	defer cancel()

	objectInCluster, err := c.kubeCl.Dynamic().
		Resource(*gvr).
		Namespace(doc.GetNamespace()).
		Get(ctx, doc.GetName(), metav1.GetOptions{})

	if err != nil {
		logNotReadyYet()
		c.logger.LogDebugF("Getting resource %s from cluster failed: %s\n", name, err)
		return false, nil
	}

	ready := checkObjectReadiness(objectInCluster, name, c.logger)

	if ready {
		c.logger.LogInfoF("Resource %s is ready!\n", name)
	} else {
		logNotReadyYet()
	}

	return ready, nil
}

func (c *resourceReadinessChecker) Name() string {
	return fmt.Sprintf("Waiting for the resource %s to become ready.", resourceName(c.resource))
}

func (c *resourceReadinessChecker) Single() bool {
	return false
}

func resourceName(r *template.Resource) string {
	result := r.GVK.String() + " '"
	if r.Object.GetNamespace() != "" {
		result = result + r.Object.GetNamespace() + "/"
	}

	return result + r.Object.GetName() + "'"
}

func checkObjectReadiness(object *unstructured.Unstructured, resourceName string, logger log.Logger) bool {
	status, ok := object.Object["status"].(map[string]interface{})
	if !ok {
		logger.LogDebugF("Resource %s do not have 'status' key. Resource ready!\n", resourceName)
		return true
	}

	// static instance case
	currentStatus, ok := status["currentStatus"].(map[string]interface{})
	if ok {
		logger.LogDebugF("Found currentStatus field. Looks like StaticInstance resource\n", resourceName)
		phase, ok := currentStatus["phase"].(string)
		if ok {
			logger.LogDebugF("Found currentStatus.phase field. Looks like StaticInstance resource\n", resourceName)
			res := phase == "Running"
			logger.LogDebugF("Found currentStatus.phase is %v. \n", resourceName, res)
			return res
		}
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		logger.LogDebugF("Resource %s do not have 'status.conditions' key. Resource ready!\n", resourceName)
		return true
	}

	for indx, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			logger.LogDebugF("Resource %s condition %d is not map. Skip. Resource ready!\n", resourceName, indx)
			continue
		}

		if conditionMap["type"] == "Ready" {
			res := conditionMap["status"] == "True"
			logger.LogDebugF("Resource %s found ready condition: %v", resourceName, res)
			return res
		}
	}

	logger.LogDebugF("Resource %s ready condition not found", resourceName)

	return false
}

func tryToGetResourceIsReadyChecker(
	kubeCl *client.KubernetesClient,
	_ *config.MetaConfig,
	r *template.Resource) (Checker, error) {

	return &resourceReadinessChecker{
		kubeCl:   kubeCl,
		resource: r,
		logger:   log.GetDefaultLogger(),
	}, nil
}
