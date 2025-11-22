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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources/readiness"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type apiResourcesGetter func(kubeCl *client.KubernetesClient, apiVersion, kind string) (*metav1.APIResource, error)

type resourceReadinessChecker struct {
	resource        *template.Resource
	resourceName    string
	resourceChecker readiness.ResourceChecker

	params          constructorParams
	getAPIResources apiResourcesGetter

	attempt        int
	cooldownPassed bool
}

func (c *resourceReadinessChecker) IsReady(ctx context.Context) (bool, error) {
	kubeCl, err := c.params.kubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return false, err
	}

	if c.getAPIResources == nil {
		return false, fmt.Errorf("Internal error. API resources getter not provided")
	}

	logger := c.params.loggerProvider()

	defer func() {
		c.attempt++
		logger.LogInfoF("\n")
	}()

	logger.LogDebugF("Resource %s readiness attempts: %d\n", c.resourceName, c.attempt)

	logger.LogInfoF("Checking if resource %s is ready...\n", c.resourceName)

	kind := c.resource.GVK.Kind

	// wait some attempts for set statuses in the resources
	if c.attempt < c.resourceChecker.WaitAttemptsBeforeCheck() {
		c.logNotReadyYet(logger)
		logger.LogDebugF("Skip resource % readiness checking for waiting set status\n", c.resourceName)
		return false, nil
	}

	c.cooldownPassed = true

	apiRes, err := c.getAPIResources(kubeCl, c.resource.GVK.Group+"/"+c.resource.GVK.Version, kind)
	if err != nil {
		logger.LogDebugF("Could not get APIResource %s with Kind %s: %s\n", c.resource.GVK.Group+"/"+c.resource.GVK.Version, kind, err.Error())
		return false, nil
	}

	gvr, doc, err := resourceToGVR(c.resource, *apiRes)
	if err != nil {
		c.logNotReadyYet(logger)
		logger.LogDebugF("Resource %s to GVR failed: %s\n", c.resourceName, err)
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objectInCluster, err := kubeCl.Dynamic().
		Resource(*gvr).
		Namespace(doc.GetNamespace()).
		Get(ctx, doc.GetName(), metav1.GetOptions{})

	if err != nil {
		c.logNotReadyYet(logger)
		logger.LogDebugF("Getting resource %s from cluster failed: %s\n", c.resourceName, err)
		return false, nil
	}

	ready, err := c.resourceChecker.IsReady(ctx, objectInCluster, c.resourceName)
	if err != nil {
		logger.LogInfoF("Readiness check for resource %s returns error: %v\n", c.resourceName, err)
		c.logNotReadyYet(logger)
		return false, nil
	}

	if ready {
		logger.LogInfoF("Resource %s is ready!\n", c.resourceName)
	} else {
		c.logNotReadyYet(logger)
	}

	return ready, nil
}

func (c *resourceReadinessChecker) Name() string {
	return fmt.Sprintf("Waiting for the resource %s to become ready.", c.resourceName)
}

func (c *resourceReadinessChecker) Single() bool {
	return false
}

func (c *resourceReadinessChecker) logNotReadyYet(logger log.Logger) {
	logger.LogInfoF("Resource %s has not been ready yet\n", c.resourceName)
}

func newResourceIsReadyChecker(r *template.Resource, params constructorParams) (*resourceReadinessChecker, error) {
	logger := params.loggerProvider()

	resourceChecker, err := readiness.GetCheckerByGvk(&r.GVK, readiness.GetCheckerParams{
		LoggerProvider: func() log.Logger {
			return logger
		},
	})

	if err != nil {
		return nil, err
	}

	resourceName := r.GVK.String() + " '"
	if r.Object.GetNamespace() != "" {
		resourceName = resourceName + r.Object.GetNamespace() + "/"
	}

	resourceName = resourceName + r.Object.GetName() + "'"

	return &resourceReadinessChecker{
		params:          params,
		resource:        r,
		resourceChecker: resourceChecker,
		resourceName:    resourceName,
		getAPIResources: func(kubeCl *client.KubernetesClient, apiVersion, kind string) (*metav1.APIResource, error) {
			return kubeCl.APIResource(apiVersion, kind)
		},
	}, nil
}

func tryToGetResourceIsReadyChecker(r *template.Resource, params constructorParams) (Checker, error) {
	return newResourceIsReadyChecker(r, params)
}
