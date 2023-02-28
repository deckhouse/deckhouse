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

package checker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/hookprobe"
)

type D8ClusterConfiguration struct {
	DeckhouseNamespace        string
	DeckhouseLabelSelector    string
	DeckhouseReadinessTimeout time.Duration

	CustomResourceName string
	Monitor            *hookprobe.Monitor
	Preflight          Doer

	Access           kubernetes.Access
	Logger           *logrus.Entry
	PreflightChecker check.Checker

	PodAccessTimeout    time.Duration
	ObjectChangeTimeout time.Duration
}

// Verify deckhouse pod is up, and running, and ready
// Set value to CR spec
// Wait for CR spec to be modified by hook
func (c *D8ClusterConfiguration) Checker() check.Checker {
	checkDeckhouse := withTimeout(
		&podRunningOrReadyChecker{
			namespace:        c.DeckhouseNamespace,
			labelSelector:    c.DeckhouseLabelSelector,
			readinessTimeout: c.DeckhouseReadinessTimeout,
			access:           c.Access,
		},
		c.PodAccessTimeout,
	)

	// Start monitor to catch the CR
	objectHandler := newHookProbeObjectHandler(c.CustomResourceName, c.Logger.WithField("component", "objectHandler"))
	c.Monitor.Subscribe(objectHandler)
	if err := c.Monitor.Start(context.Background()); err != nil {
		panic(fmt.Errorf("cannot start monitor: %v", err))
	}

	gvr := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "upmeterhookprobes",
	}
	dynamicClient := c.Access.Kubernetes().Dynamic().Resource(gvr)

	setInitedValue := withTimeout(
		newSetInitedValueChecker(dynamicClient, c.CustomResourceName, c.Logger.WithField("component", "setter")),
		c.ObjectChangeTimeout,
	)

	checkMirrorValue := withRetryEachSeconds(
		&checkMirrorValueChecker{
			name:   c.CustomResourceName,
			getter: objectHandler,
			logger: c.Logger.WithField("component", "verifier"),
		},
		c.ObjectChangeTimeout,
	)

	return sequence(
		c.PreflightChecker,
		checkDeckhouse,
		setInitedValue,
		checkMirrorValue,
	)
}

func newHookProbeObjectHandler(name string, logger *logrus.Entry) *HookProbeHandler {
	return &HookProbeHandler{
		name:   name,
		logger: logger,
	}
}

type HookProbeHandler struct {
	name   string
	logger *logrus.Entry

	// Inner state
	obj *hookprobe.HookProbe
	mu  sync.RWMutex
}

func (h *HookProbeHandler) Get() *hookprobe.HookProbe {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.obj
}

func (h *HookProbeHandler) OnAdd(obj *hookprobe.HookProbe) {
	h.logger.Debug("object added")
	h.OnModify(obj)
}

func (h *HookProbeHandler) OnModify(obj *hookprobe.HookProbe) {
	if obj.GetName() != h.name {
		return
	}
	h.logger.Debug("object modified")

	h.mu.Lock()
	defer h.mu.Unlock()

	h.obj = obj
}

func (h *HookProbeHandler) OnDelete(obj *hookprobe.HookProbe) {
	if obj.GetName() != h.name {
		return
	}
	h.logger.Debug("object deleted")

	h.mu.Lock()
	defer h.mu.Unlock()

	h.obj = nil
}

func newSetInitedValueChecker(dynamicClient dynamic.ResourceInterface, name string, logger *logrus.Entry) *setInitedValueChecker {
	template := `
apiVersion: deckhouse.io/v1
kind: UpmeterHookProbe
metadata:
  name: %q
  labels:
    app: upmeter
    heritage: upmeter
    upmeter-agent: %q
    upmeter-group: deckhouse
    upmeter-probe: cluster-configuration
spec:
  inited: %q
  mirror: "<empty>"
`

	return &setInitedValueChecker{
		name:          name,
		template:      template,
		dynamicClient: dynamicClient,
		fieldManager:  "upmeter-agent-" + name,
		logger:        logger,
	}
}

type setInitedValueChecker struct {
	name          string
	template      string
	fieldManager  string
	dynamicClient dynamic.ResourceInterface
	logger        *logrus.Entry
}

func (c *setInitedValueChecker) Check() check.Error {
	newValue := string(uuid.NewUUID())

	obj, err := c.dynamicClient.Get(context.TODO(), c.name, metav1.GetOptions{})
	if err != nil || obj == nil {
		c.logger.Debugf("creating object with value %s", newValue)

		return c.create(newValue)
	}

	c.logger.Debugf("updating object with value %s", newValue)

	return c.update(obj, newValue)
}

func (c *setInitedValueChecker) update(obj *unstructured.Unstructured, value string) check.Error {
	if err := unstructured.SetNestedField(obj.Object, value, "spec", "inited"); err != nil {
		return check.ErrFail("cannot set new inited value to UpmeterHookProbe object %q at runtime: %v", c.name, err)
	}

	opts := metav1.UpdateOptions{FieldManager: c.fieldManager}
	if _, err := c.dynamicClient.Update(context.TODO(), obj, opts); err != nil {
		return check.ErrFail("cannot update UpmeterHookProbe object %q with new inited value: %v", c.name, err)
	}

	return nil
}

func (c *setInitedValueChecker) create(value string) check.Error {
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	obj := &unstructured.Unstructured{}
	objName, agentID := c.name, c.name
	manifest := fmt.Sprintf(c.template, objName, agentID, value)

	if _, _, err := decUnstructured.Decode([]byte(manifest), nil, obj); err != nil {
		return check.ErrFail("cannot create UpmeterHookProbe object at runtime: %v", err)
	}

	opts := metav1.CreateOptions{FieldManager: c.fieldManager}
	if _, err := c.dynamicClient.Create(context.TODO(), obj, opts); err != nil {
		return check.ErrFail("cannot create UpmeterHookProbe object in cluster: %v", err)
	}

	return nil
}

type hookProbeObjectGetter interface {
	Get() *hookprobe.HookProbe
}

type checkMirrorValueChecker struct {
	name   string
	getter hookProbeObjectGetter
	logger *logrus.Entry
}

func (c *checkMirrorValueChecker) Check() check.Error {
	c.logger.Debug("fetching object")

	obj := c.getter.Get()
	if obj == nil {
		return check.ErrFail("no CR object")
	}

	c.logger.WithFields(map[string]interface{}{"inited": obj.Spec.Inited, "mirror": obj.Spec.Mirror}).Debug("object fetched")

	if obj.Spec.Inited != obj.Spec.Mirror {
		return check.ErrFail(
			"object values are not the same: inited=%s, mirror=%s",
			obj.Spec.Inited, obj.Spec.Mirror,
		)
	}
	return nil
}
