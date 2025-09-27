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

	waitingConditionAttempts int
}

// kind should be in lower case!
var kindsToAttempts = map[string]int{
	// in some cases deckhouse controller doesn't have time for set status field, and we have Ready nodegroup whe it is not Ready
	"nodegroup": 5,
}

func resourceName(r *template.Resource) string {
	result := r.GVK.String() + " '"
	if r.Object.GetNamespace() != "" {
		result = result + r.Object.GetNamespace() + "/"
	}

	return result + r.Object.GetName() + "'"
}

func (c *resourceReadinessChecker) IsReady(ctx context.Context) (bool, error) {
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
	apiRes, err := c.kubeCl.APIResource(c.resource.GVK.Group+"/"+c.resource.GVK.Version, kind)
	if err != nil {
		c.logger.LogDebugF("Could not get APIResource %s with Kind %s: %s\n", c.resource.GVK.Group+"/"+c.resource.GVK.Version, kind, err.Error())
		return false, nil
	}

	gvr, doc, err := resourceToGVR(c.resource, *apiRes)
	if err != nil {
		logNotReadyYet()
		c.logger.LogDebugF("Resource %s to GVR failed: %s\n", name, err)
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
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

	ready := c.checkObjectReadiness(objectInCluster, name)

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

func (c *resourceReadinessChecker) checkObjectReadiness(object *unstructured.Unstructured, resourceName string) bool {
	logger := c.logger

	status, ok := object.Object["status"].(map[string]interface{})
	if !ok {
		logger.LogDebugF("Resource %s do not have 'status' key. Resource ready!\n", resourceName)
		return true
	}

	// static instance case
	currentStatus, ok := status["currentStatus"].(map[string]interface{})
	if ok {
		logger.LogDebugF("Found currentStatus field. Looks like %s StaticInstance resource\n", resourceName)
		phase, ok := currentStatus["phase"].(string)
		if ok {
			logger.LogDebugF("Found currentStatus.phase field. Looks like %s is StaticInstance resource\n", resourceName)
			res := phase == "Running"
			logger.LogDebugF("Found for %s currentStatus.phase is %v. \n", resourceName, res)
			return res
		}
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		logger.LogDebugF("Resource %s do not have 'status.conditions' key. Resource ready!\n", resourceName)
		return true
	}

	isTrue := func(conditionMap map[string]interface{}, t string, indx int) bool {
		stat, ok := conditionMap["status"].(string)
		if !ok {
			logger.LogDebugF("Resource %s condition %d status is not string. Skip. Resource ready!\n", resourceName, indx)
			return true
		}

		res := stat == "True"

		logger.LogDebugF("Resource %s found `%s` condition: %v", resourceName, t, res)

		return res
	}

	// We only expect two conditions: Ready and Available. This will work well with most resources, such as NodeGroup Deployment ApiService.
	// But we won't consider Job here, since conditions only appear after completion and error, and we don't want to complicate the detection logic yet.
	for indx, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			logger.LogDebugF("Resource %s condition %d is not map. Skip. Resource ready!\n", resourceName, indx)
			return true
		}

		tp, ok := conditionMap["type"].(string)
		if !ok {
			logger.LogDebugF("Resource %s condition %d type is not string. Skip. Resource ready!\n", resourceName, indx)
			return true
		}

		switch tp {
		// Pod, NodeGroup and thousands them
		case "Ready":
			return isTrue(conditionMap, "Ready", indx)
		// Deployment, APIService
		case "Available":
			return isTrue(conditionMap, "Available", indx)
		}
	}

	c.waitingConditionAttempts++
	const attemptsLimit = 5

	if c.waitingConditionAttempts <= attemptsLimit {
		logger.LogDebugF("Resource %s support conditions not found. Attempt %d/%d", resourceName, c.waitingConditionAttempts, attemptsLimit)
		return false
	}

	// We think so because each CRD can have its own Ready condition for the resource,
	// and we will not be able to cover them all, so we simply accept the resource as is.
	logger.LogDebugF("Resource %s support condition not found. Attempts limit exceeded. We believe that the resource is ready", resourceName)
	return true
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
