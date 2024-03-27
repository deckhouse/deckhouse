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
	"time"

	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	t "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type clusterStore struct {
	dc  *dynamic.DynamicClient
	GVR schema.GroupVersionResource
}

func newClusterStore() *clusterStore {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	k8sClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatal(err)
	}

	return &clusterStore{
		dc: k8sClient,
		GVR: schema.GroupVersionResource{
			Group:    "deckhouse.io",
			Version:  "v1alpha1",
			Resource: "clusteralerts",
		},
	}
}

func (c *clusterStore) listCRs(rootCtx context.Context) (map[string]struct{}, error) {
	log.Info("list CRs in the cluster")
	ctx, cancel := context.WithTimeout(rootCtx, contextTimeout)
	crList, err := c.dc.Resource(c.GVR).List(ctx, v1.ListOptions{
		LabelSelector:        fmt.Sprintf("app=%s,heritage=deckhouse", appName),
		ResourceVersionMatch: v1.ResourceVersionMatchNotOlderThan,
		ResourceVersion:      "0",
	})
	cancel()
	if err != nil {
		return nil, err
	}
	res := make(map[string]struct{}, len(crList.Items))
	for _, item := range crList.Items {
		res[item.GetName()] = struct{}{}
	}
	log.Infof("found %d CRs in cluster", len(crList.Items))
	return res, nil
}

// Remove CR from cluster
func (c *clusterStore) removeCR(rootCtx context.Context, fingerprint string) error {
	log.Infof("remove CR with name %s from cluster", fingerprint)
	ctx, cancel := context.WithTimeout(rootCtx, contextTimeout)
	err := c.dc.Resource(c.GVR).Delete(ctx, fingerprint, v1.DeleteOptions{})
	cancel()
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// Create CR for corresponding alert in cluster
func (c *clusterStore) createCR(rootCtx context.Context, fingerprint string, alert *types.Alert) error {
	log.Infof("creating CR with name %s", fingerprint)

	severityLevel := getLabel(alert.Labels, severityLabel)
	summary := getLabel(alert.Annotations, summaryLabel)
	description := getLabel(alert.Annotations, descriptionLabel)

	reducedAnnotations := alert.Annotations.Clone()
	reducedLabels := alert.Labels.Clone()

	// remove unnecessary fields
	delete(reducedAnnotations, summaryLabel)
	delete(reducedAnnotations, descriptionLabel)
	removePlkAnnotations(reducedAnnotations)

	delete(reducedLabels, severityLabel)
	delete(reducedLabels, model.AlertNameLabel)

	a := &ClusterAlert{
		TypeMeta: v1.TypeMeta{
			APIVersion: "deckhouse.io/v1alpha1",
			Kind:       "ClusterAlert",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:   fingerprint,
			Labels: map[string]string{"app": appName, "heritage": "deckhouse"},
		},
		Alert: ClusterAlertSpec{
			Name:          alert.Name(),
			SeverityLevel: severityLevel,
			Summary:       summary,
			Description:   description,
			Annotations:   reducedAnnotations,
			Labels:        reducedLabels,
		},
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(a)
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	obj.Object = content
	ctx, cancel := context.WithTimeout(rootCtx, contextTimeout)
	_, err = c.dc.Resource(c.GVR).Create(ctx, obj, v1.CreateOptions{})
	cancel()

	if err != nil {
		return err
	}

	// if alert.StartsAt time is zero, set status startsAt field to now on CR create
	startsAt := alert.StartsAt
	if startsAt.IsZero() {
		startsAt = time.Now()
	}

	err = c.updateCRStatus(rootCtx, fingerprint, startsAt, alert.UpdatedAt)
	return err
}

// Uodate CR status
func (c *clusterStore) updateCRStatus(rootCtx context.Context, fingerprint string, startsAt, updatedAt time.Time) error {
	log.Infof("update status of CR with name %s", fingerprint)

	alertStatus := clusterAlertFiring

	// If alert was updated last time > 2min ago, alert is marked as stale
	if time.Since(updatedAt) > 2*reconcileTime {
		alertStatus = clusterAlertFiringStaled
	}

	statusField := map[string]interface{}{
		"alertStatus":    alertStatus,
		"lastUpdateTime": updatedAt.Format(time.RFC3339),
	}
	// id StartsAt field of alert is zero, don't update startsAt status field
	if !startsAt.IsZero() {
		statusField["startsAt"] = startsAt.Format(time.RFC3339)
	}

	patch := map[string]interface{}{
		"status": statusField,
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(rootCtx, contextTimeout)
	_, err = c.dc.Resource(c.GVR).Patch(ctx, fingerprint, t.MergePatchType, data, v1.PatchOptions{}, "/status")
	cancel()
	return err
}

// Return label by key as string
func getLabel(labels model.LabelSet, key string) string {
	return string(labels[model.LabelName(key)])
}

// Remove unwanted annotations started with plk_
func removePlkAnnotations(annotations model.LabelSet) {
	for k := range annotations {
		if strings.HasPrefix(string(k), "plk_") {
			delete(annotations, k)
		}
	}
}
