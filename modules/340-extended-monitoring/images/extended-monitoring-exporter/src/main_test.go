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
	"encoding/json"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fake "k8s.io/client-go/metadata/fake"
)

var (
	ns          = metav1.TypeMeta{APIVersion: "v1", Kind: "namespaces"}
	nodes       = metav1.TypeMeta{APIVersion: "v1", Kind: "nodes"}
	pods        = metav1.TypeMeta{APIVersion: "v1", Kind: "pods"}
	ingress     = metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"}
	deployment  = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}
	daemonset   = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"}
	statefulset = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"}
	cronjob     = metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"}
)

var (
	enabled   = "extended-monitoring.deckhouse.io/enabled"
	threshold = "threshold.extended-monitoring.deckhouse.io/"
)

func removeCreatedTimestamp(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if key == "created_timestamp" {
				continue
			}
			result[key] = removeCreatedTimestamp(value)
		}
		return result
	case []interface{}:
		for i, item := range v {
			v[i] = removeCreatedTimestamp(item)
		}
		return v
	default:
		return v
	}
}

func cleanedJSON(t *testing.T, client *fake.FakeMetadataClient) string {
	registry := prometheus.NewRegistry()
	recordMetrics(context.Background(), client, registry)
	mfs, err := registry.Gather()
	assert.NoError(t, err, "Error gathering metrics")
	mfsJSON, err := json.Marshal(mfs)
	assert.NoError(t, err, "Error converte to JSON")
	var parsedData interface{}
	err = json.Unmarshal(mfsJSON, &parsedData)
	assert.NoError(t, err, "Error gathering mfsJSON")
	cleanedData := removeCreatedTimestamp(parsedData)
	cleanedJSON, err := json.Marshal(cleanedData)
	assert.NoError(t, err, "Error converte to JSON")
	return string(cleanedJSON)
}

func createResource(client *fake.FakeMetadataClient, resource schema.GroupVersionResource, namespace string, meta metav1.TypeMeta, object metav1.ObjectMeta) error {
	var request fake.MetadataClient
	if namespace != "" {
		request = client.Resource(resource).Namespace(namespace).(fake.MetadataClient)
	} else {
		request = client.Resource(resource).(fake.MetadataClient)
	}
	_, err := request.CreateFake(&metav1.PartialObjectMetadata{
		TypeMeta:   meta,
		ObjectMeta: object,
	}, metav1.CreateOptions{})
	return err
}

func TestEnabledLabel(t *testing.T) {
	labels := map[string]string{enabled: "true"}
	assert.Equal(t, 1.0, enabledLabel(labels))

	labels[enabled] = "false"
	assert.Equal(t, 0.0, enabledLabel(labels))

	delete(labels, enabled)
	assert.Equal(t, 1.0, enabledLabel(labels))
}

func TestThresholdLabel(t *testing.T) {
	labels := map[string]string{threshold + "cpu": "80"}
	assert.Equal(t, 80.0, thresholdLabel(labels, "cpu", 100.0))

	labels[threshold+"cpu"] = "invalid"
	assert.Equal(t, 100.0, thresholdLabel(labels, "cpu", 100.0))
}

func TestMetricsEnabled(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"namespace1"}]},
				{"counter":{"value":0},"label":[{"name":"namespace","value":"namespace2"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)

	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "namespace1",
		Labels: map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "namespace2",
		Labels: map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "namespace3",
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsNode(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_node_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"node","value":"node1"}]},
				{"counter":{"value":0},"label":[{"name":"node","value":"node2"}]}
		]},{
			"name":"extended_monitoring_node_threshold","type":0,"help":"","metric":[
				{"counter":{"value":80},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"disk-bytes-critical"}]},
				{"counter":{"value":70},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"disk-bytes-warning"}]},
				{"counter":{"value":95},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"disk-inodes-critical"}]},
				{"counter":{"value":90},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"disk-inodes-warning"}]},
				{"counter":{"value":9},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"load-average-per-core-critical"}]},
				{"counter":{"value":3},"label":[{"name":"node","value":"node1"},{"name":"threshold","value":"load-average-per-core-warning"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNodes, "", ns, metav1.ObjectMeta{
		Name:   "node1",
		Labels: map[string]string{threshold + "load-average-per-core-critical": "9"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNodes, "", ns, metav1.ObjectMeta{
		Name:   "node2",
		Labels: map[string]string{enabled: "false"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsPod(t *testing.T) {
	testJSON := `[
		{
			"name": "extended_monitoring_enabled","help": "","type": 0,"metric": [
				{"label":[{"name": "namespace","value": "ns1"}],"counter":{"value": 1}}
		]},{
			"name": "extended_monitoring_pod_enabled","help": "","type": 0,"metric": [
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod1"}],"counter": {"value": 0}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"}],"counter": {"value": 1}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"}],"counter": {"value": 1}}
		]},{
			"name": "extended_monitoring_pod_threshold","help": "","type": 0,"metric": [
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "container-throttling-critical"}],"counter": {"value": 50}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "container-throttling-warning"}],"counter": {"value": 25}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "disk-bytes-critical"}],"counter": {"value": 94}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "disk-bytes-warning"}],"counter": {"value": 84}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "disk-inodes-critical"}],"counter": {"value": 90}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod2"},{"name": "threshold", "value": "disk-inodes-warning"}],"counter": {"value": 85}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "container-throttling-critical"}],"counter": {"value": 50}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "container-throttling-warning"}],"counter": {"value": 25}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "disk-bytes-critical"}],"counter": {"value": 94}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "disk-bytes-warning"}],"counter": {"value": 85}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "disk-inodes-critical"}],"counter": {"value": 90}},
				{"label": [{"name": "namespace", "value": "ns1"},{"name": "pod", "value": "pod3"},{"name": "threshold", "value": "disk-inodes-warning"}],"counter": {"value": 85}}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true", threshold + "disk-bytes-critical": "94"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourcePods, "ns1", pods, metav1.ObjectMeta{
		Name:      "pod1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourcePods, "ns1", pods, metav1.ObjectMeta{
		Name:      "pod2",
		Namespace: "ns1",
		Labels:    map[string]string{threshold + "disk-bytes-warning": "84"},
	}))
	assert.NoError(t, createResource(FakeClient, resourcePods, "ns1", pods, metav1.ObjectMeta{
		Name:      "pod3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourcePods, "ns2", pods, metav1.ObjectMeta{
		Name:      "pod4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsIngress(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_enabled","help":"","type":0,"metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"}]}
		]},{
			"name":"extended_monitoring_ingress_enabled","type":0,"help":"","metric":[
				{"counter":{"value":0},"label":[{"name":"ingress","value":"ing1"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"ingress","value":"ing2"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"ingress","value":"ing3"},{"name":"namespace","value":"ns1"}]}
			]	
		},{
			"name":"extended_monitoring_ingress_threshold","type":0,"help":"","metric":[
				{"counter":{"value":22},"label":[{"name":"ingress","value":"ing2"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"5xx-critical"}]},
				{"counter":{"value":9},"label":[{"name":"ingress","value":"ing2"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"5xx-warning"}]},
				{"counter":{"value":22},"label":[{"name":"ingress","value":"ing3"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"5xx-critical"}]},
				{"counter":{"value":10},"label":[{"name":"ingress","value":"ing3"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"5xx-warning"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true", threshold + "5xx-critical": "22"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourceIngresses, "ns1", ingress, metav1.ObjectMeta{
		Name:      "ing1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceIngresses, "ns1", ingress, metav1.ObjectMeta{
		Name:      "ing2",
		Namespace: "ns1",
		Labels:    map[string]string{threshold + "5xx-warning": "9"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceIngresses, "ns1", ingress, metav1.ObjectMeta{
		Name:      "ing3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceIngresses, "ns2", ingress, metav1.ObjectMeta{
		Name:      "ing4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsDeployment(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_deployment_enabled","type":0,"help":"","metric":[
				{"counter":{"value":0},"label":[{"name":"deployment","value":"deploy1"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"deployment","value":"deploy2"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"deployment","value":"deploy3"},{"name":"namespace","value":"ns1"}]}
		]},{
			"name":"extended_monitoring_deployment_threshold","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"deployment","value":"deploy2"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"replicas-not-ready"}]},
				{"counter":{"value":2},"label":[{"name":"deployment","value":"deploy3"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"replicas-not-ready"}]}
		]},{
			"name":"extended_monitoring_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true", threshold + "replicas-not-ready": "2"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourceDeployments, "ns1", deployment, metav1.ObjectMeta{
		Name:      "deploy1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDeployments, "ns1", deployment, metav1.ObjectMeta{
		Name:      "deploy2",
		Namespace: "ns1",
		Labels:    map[string]string{threshold + "replicas-not-ready": "1"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDeployments, "ns1", deployment, metav1.ObjectMeta{
		Name:      "deploy3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDeployments, "ns2", deployment, metav1.ObjectMeta{
		Name:      "deploy4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsDaemonset(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_daemonset_enabled","type":0,"help":"","metric":[
				{"counter":{"value":0},"label":[{"name":"daemonset","value":"ds1"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"daemonset","value":"ds2"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"daemonset","value":"ds3"},{"name":"namespace","value":"ns1"}]}
		]},{
			"name":"extended_monitoring_daemonset_threshold","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"daemonset","value":"ds2"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"replicas-not-ready"}]},
				{"counter":{"value":2},"label":[{"name":"daemonset","value":"ds3"},{"name":"namespace","value":"ns1"},{"name":"threshold","value":"replicas-not-ready"}]}
		]},{
			"name":"extended_monitoring_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true", threshold + "replicas-not-ready": "2"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourceDaemonsets, "ns1", daemonset, metav1.ObjectMeta{
		Name:      "ds1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDaemonsets, "ns1", daemonset, metav1.ObjectMeta{
		Name:      "ds2",
		Namespace: "ns1",
		Labels:    map[string]string{threshold + "replicas-not-ready": "1"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDaemonsets, "ns1", daemonset, metav1.ObjectMeta{
		Name:      "ds3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceDaemonsets, "ns2", daemonset, metav1.ObjectMeta{
		Name:      "ds4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsStatefulset(t *testing.T) {
	testJSON := `[
		{	
			"name":"extended_monitoring_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"}]}
		]},{
			"name":"extended_monitoring_statefulset_enabled","type":0,"help":"","metric":[
				{"counter":{"value":0},"label":[{"name":"namespace","value":"ns1"},{"name":"statefulset","value":"sts1"}]},
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"},{"name":"statefulset","value":"sts2"}]},
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"},{"name":"statefulset","value":"sts3"}]}
		]},{
			"name":"extended_monitoring_statefulset_threshold","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"},{"name":"statefulset","value":"sts2"},{"name":"threshold","value":"replicas-not-ready"}]},
				{"counter":{"value":2},"label":[{"name":"namespace","value":"ns1"},{"name":"statefulset","value":"sts3"},{"name":"threshold","value":"replicas-not-ready"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true", threshold + "replicas-not-ready": "2"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourceStatefulsets, "ns1", statefulset, metav1.ObjectMeta{
		Name:      "sts1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceStatefulsets, "ns1", statefulset, metav1.ObjectMeta{
		Name:      "sts2",
		Namespace: "ns1",
		Labels:    map[string]string{threshold + "replicas-not-ready": "1"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceStatefulsets, "ns1", statefulset, metav1.ObjectMeta{
		Name:      "sts3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceStatefulsets, "ns2", statefulset, metav1.ObjectMeta{
		Name:      "sts4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}

func TestMetricsCronjob(t *testing.T) {
	testJSON := `[
		{
			"name":"extended_monitoring_cronjob_enabled","type":0,"help":"","metric":[
				{"counter":{"value":0},"label":[{"name":"cronjob","value":"cron1"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"cronjob","value":"cron2"},{"name":"namespace","value":"ns1"}]},
				{"counter":{"value":1},"label":[{"name":"cronjob","value":"cron3"},{"name":"namespace","value":"ns1"}]}
		]},{
			"name":"extended_monitoring_enabled","type":0,"help":"","metric":[
				{"counter":{"value":1},"label":[{"name":"namespace","value":"ns1"}]}
		]}]`

	scheme := runtime.NewScheme()
	_ = metav1.AddMetaToScheme(scheme)
	FakeClient := fake.NewSimpleMetadataClient(scheme)
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name:   "ns1",
		Labels: map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceNamespaces, "", ns, metav1.ObjectMeta{
		Name: "ns2",
	}))
	assert.NoError(t, createResource(FakeClient, resourceCronjobs, "ns1", cronjob, metav1.ObjectMeta{
		Name:      "cron1",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "false"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceCronjobs, "ns1", cronjob, metav1.ObjectMeta{
		Name:      "cron2",
		Namespace: "ns1",
	}))
	assert.NoError(t, createResource(FakeClient, resourceCronjobs, "ns1", cronjob, metav1.ObjectMeta{
		Name:      "cron3",
		Namespace: "ns1",
		Labels:    map[string]string{enabled: "true"},
	}))
	assert.NoError(t, createResource(FakeClient, resourceCronjobs, "ns2", cronjob, metav1.ObjectMeta{
		Name:      "cron4",
		Namespace: "ns2",
		Labels:    map[string]string{enabled: "true"},
	}))

	assert.JSONEq(t, testJSON, cleanedJSON(t, FakeClient))
}
