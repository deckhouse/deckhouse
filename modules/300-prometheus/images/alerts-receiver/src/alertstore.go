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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	t "k8s.io/apimachinery/pkg/types"
)

type alertStoreStruct struct {
	capacity int

	sync.RWMutex
	alerts map[model.Fingerprint]*types.Alert
}

func newStore(l int) *alertStoreStruct {
	a := make(map[model.Fingerprint]*types.Alert, l)
	return &alertStoreStruct{alerts: a, capacity: l}
}

// Add or update alert in internal store
func (a *alertStoreStruct) insertAlert(alert *model.Alert) {
	a.Lock()
	defer a.Unlock()

	now := time.Now()

	removePlkAnnotations(alert)

	ta := &types.Alert{
		Alert:     *alert,
		UpdatedAt: now,
	}

	// Ensure StartsAt is set.
	if ta.StartsAt.IsZero() {
		if ta.EndsAt.IsZero() {
			ta.StartsAt = now
		} else {
			ta.StartsAt = ta.EndsAt
		}
	}
	// If no end time is defined, set a timeout after which an alert
	// is marked resolved if it is not updated.
	if ta.EndsAt.IsZero() {
		ta.Timeout = true
		ta.EndsAt = now.Add(resolveTimeout)
	}
	fingerprint := ta.Fingerprint()

	if _, ok := a.alerts[fingerprint]; ok {
		log.Infof("alert with fingerprint %s updated in queue", fingerprint)
		a.alerts[fingerprint] = ta.Merge(a.alerts[fingerprint])
	} else {
		log.Infof("alert with fingerprint %s added to queue", fingerprint)
	}
	a.alerts[fingerprint] = ta

	return
}

// Remove alert from internal store
func (a *alertStoreStruct) removeAlert(fingerprint model.Fingerprint) {
	a.Lock()
	defer a.Unlock()
	log.Infof("alert with fingerprint %s removed from queue", fingerprint)
	delete(a.alerts, fingerprint)
}

func (a *alertStoreStruct) insertCR(fingerprint model.Fingerprint) error {
	a.RLock()
	defer a.RUnlock()

	log.Infof("creating CR with name %s", fingerprint)

	alert := &ClusterAlert{
		TypeMeta: v1.TypeMeta{
			APIVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterAlert",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:   fingerprint.String(),
			Labels: map[string]string{"app": appName, "heritage": "deckhouse"},
		},
		Alert: ClusterAlertSpec{
			Name:          a.alerts[fingerprint].Name(),
			SeverityLevel: getLabel(a.alerts[fingerprint].Labels, severityLabel),
			Summary:       getLabel(a.alerts[fingerprint].Annotations, summaryLabel),
			Description:   getLabel(a.alerts[fingerprint].Annotations, descriptionLabel),
			Annotations:   a.alerts[fingerprint].Annotations,
			Labels:        a.alerts[fingerprint].Labels,
		},
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(alert)
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	obj.Object = content
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	_, err = config.k8sClient.Resource(GVR).Create(ctx, obj, v1.CreateOptions{})
	cancel()

	return err
}

// Uodate CR status
func (a *alertStoreStruct) updateCRStatus(fingerprint model.Fingerprint) error {
	a.RLock()
	defer a.RUnlock()

	log.Infof("update status of CR with name %s", fingerprint)

	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"alertStatus":    clusterAlertFiring,
			"startsAt":       fmt.Sprintf("%d", a.alerts[fingerprint].StartsAt.UnixMicro()),
			"lastUpdateTime": fmt.Sprintf("%d", a.alerts[fingerprint].UpdatedAt.UnixMicro()),
		},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	_, err = config.k8sClient.Resource(GVR).Patch(ctx, fingerprint.String(), t.MergePatchType, data, v1.PatchOptions{}, "/status")
	cancel()
	return err
}

// Return label by key as string
func getLabel(labels model.LabelSet, key string) string {
	return string(labels[model.LabelName(key)])
}

// Remove unwanted annotations started with plk_
func removePlkAnnotations(alert *model.Alert) {
	for k := range alert.Annotations {
		if strings.HasPrefix(string(k), "plk_") {
			delete(alert.Annotations, k)
		}
	}
}
