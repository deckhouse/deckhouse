/*
Copyright 2023 Flant JSC

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

package readyz

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// DeploymentReadinessCheck is a health check tied to a readiness probe status of other deployment.
type DeploymentReadinessCheck struct {
	deployInformer  cache.SharedIndexInformer
	namespace, name string

	mu                 sync.RWMutex // guards the deployment state
	currentDeployState *v1.Deployment
}

func NewDeploymentReadinessCheck(
	stopCh <-chan struct{},
	deployInformer cache.SharedIndexInformer,
	namespace, name string,
) (*DeploymentReadinessCheck, error) {
	d := &DeploymentReadinessCheck{
		deployInformer: deployInformer,
		namespace:      namespace,
		name:           name,
	}

	d.deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    d.onDeployAdded,
		UpdateFunc: d.onDeployUpdate,
		DeleteFunc: d.onDeployDelete,
	})

	go d.deployInformer.Run(stopCh)

	backoff := wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Jitter:   0,
		Steps:    10,
	}
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		return d.deployInformer.HasSynced(), nil
	})
	switch {
	case errors.Is(err, wait.ErrWaitTimeout):
		return nil, fmt.Errorf("waiting for informer to sync: %w", err)
	case err != nil:
		return nil, fmt.Errorf("wait.ExponentialBackoffWithContext: %w", err)
	}

	return d, nil
}

func (d *DeploymentReadinessCheck) Name() string {
	return fmt.Sprintf("%s-%s/%s", reflect.ValueOf(d).Elem().Type().Name(), d.namespace, d.name)
}

func (d *DeploymentReadinessCheck) Check(_ *http.Request) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.currentDeployState == nil {
		return nil // Deployment doesn't exist, probably a bootstrap phase.
	}

	if d.currentDeployState.Status.ReadyReplicas == 0 {
		return fmt.Errorf("bashible-apiserver is postponed until %s/%s deployment is ready", d.namespace, d.name)
	}

	return nil
}

func (d *DeploymentReadinessCheck) onDeployAdded(obj interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	deploy := obj.(*v1.Deployment)
	if deploy.Namespace == d.namespace && deploy.Name == d.name {
		d.currentDeployState = deploy
	}
}

func (d *DeploymentReadinessCheck) onDeployUpdate(_, newObj interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	deploy := newObj.(*v1.Deployment)
	if deploy.Namespace == d.namespace && deploy.Name == d.name {
		d.currentDeployState = deploy
	}
}

func (d *DeploymentReadinessCheck) onDeployDelete(obj interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	deploy := obj.(*v1.Deployment)
	if deploy.Namespace == d.namespace && deploy.Name == d.name {
		d.currentDeployState = nil
	}
}
