/*
Copyright 2025 Flant JSC

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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fake "k8s.io/client-go/metadata/fake"
)

func TestEnabledLabel(t *testing.T) {
	labels := map[string]string{namespaces_enabled_label: "true"}
	assert.Equal(t, 1.0, enabledLabel(labels))

	labels[namespaces_enabled_label] = "false"
	assert.Equal(t, 0.0, enabledLabel(labels))

	delete(labels, namespaces_enabled_label)
	assert.Equal(t, 1.0, enabledLabel(labels))
}

func TestThresholdLabel(t *testing.T) {
	labels := map[string]string{label_theshold_prefix + "cpu": "80"}
	assert.Equal(t, 80.0, thresholdLabel(labels, "cpu", 100.0))

	labels[label_theshold_prefix+"cpu"] = "invalid"
	assert.Equal(t, 100.0, thresholdLabel(labels, "cpu", 100.0))
}

func TestRecordMetrics(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	FakeClient.Resource(resource_namespaces).(fake.MetadataClient).CreateFake(&metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "namespaces",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "namespace1",
			Labels: map[string]string{namespaces_enabled_label: "true"},
		},
	}, metav1.CreateOptions{})

	// FakeClient.Resource(resource_namespaces).(fake.MetadataClient).CreateFake(&metav1.PartialObjectMetadata{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: "v1",
	// 		Kind:       "pod",
	// 	},
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "pod1",
	// 		Namespace: "namespace1",
	// 	},
	// }, metav1.CreateOptions{})

	registry := prometheus.NewRegistry()
	recordMetrics(ctx, FakeClient, registry)

	mfs, err := registry.Gather()
	if err != nil {
		t.Fatalf("Error gathering metrics: %v", err)
	}
	assert.Equal(t, 1, len(mfs))
	assert.Equal(t, "extended_monitoring_enabled", mfs[0].GetName())
	assert.Regexp(t, "^name:\"extended_monitoring_enabled\".*help:\"\".*type:COUNTER.*metric:{label:{name:\"namespace\".*value:\"namespace1\"}.*counter:{value:1.*created_timestamp:.*$", mfs[0].String())

}
