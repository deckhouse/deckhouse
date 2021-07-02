/*
Copyright 2021 Flant CJSC

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

package hooks

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "daemonset",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc: ApplyDaemonSetControllerFilter,
		},
	},
}, dependency.WithExternalDependencies(migrateControllerBeforeHelm))

type DaemonSetController struct {
	Name   string                 `json:"name"`
	Status appsv1.DaemonSetStatus `json:"status"`
}

func ApplyDaemonSetControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &appsv1.DaemonSet{}

	err := sdk.FromUnstructured(obj, ds)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	if !strings.HasPrefix(ds.Annotations["ingress-nginx-controller.deckhouse.io/inlet"], "LoadBalancer") {
		return nil, nil
	}

	return DaemonSetController{
		Name:   ds.Labels["name"],
		Status: ds.Status,
	}, nil
}

func migrateControllerBeforeHelm(input *go_hook.HookInput, dc dependency.Container) (err error) {
	daemonsets := input.Snapshots["daemonset"]

	for _, ds := range daemonsets {
		if ds == nil {
			continue
		}
		controller := ds.(DaemonSetController)

		minReplicas := controller.Status.DesiredNumberScheduled * 2
		maxReplicas := minReplicas * 3

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"minReplicas": minReplicas,
				"maxReplicas": maxReplicas,
			},
		}
		data, _ := json.Marshal(patch)

		err := input.ObjectPatcher.MergePatchObject(data, "deckhouse.io/v1", "IngressNginxController", "", controller.Name, "")
		if err != nil {
			return fmt.Errorf("patch IngressNginxController %q failed: %v", controller.Name, err)
		}

		err = removeControllerDaemonSetFromHelmRelease(controller.Name, dc)
		if err != nil {
			return fmt.Errorf("remove ds from helm release for controller %q failed: %v", controller.Name, err)
		}
	}

	return nil
}

func ParseReleaseSecretToJSON(secret *v1.Secret) (map[string]interface{}, error) {
	release, err := base64.StdEncoding.DecodeString(string(secret.Data["release"]))
	if err != nil {
		return nil, fmt.Errorf("error decoding release data from base64: %v", err)
	}
	if len(release) == 0 {
		return nil, fmt.Errorf("got zero length release data")
	}
	uncompressedRelease, err := gUnzipData(release)
	if err != nil {
		return nil, fmt.Errorf("error uncompressing release data: %v", err)
	}
	if len(uncompressedRelease) == 0 {
		return nil, fmt.Errorf("got zero length uncompressed release data")
	}

	var jsonRelease map[string]interface{}
	err = json.Unmarshal(uncompressedRelease, &jsonRelease)
	if err != nil {
		return nil, fmt.Errorf("got error unmarshalling release data: %v", err)
	}

	return jsonRelease, nil
}

func appendNodeToManifestIfItNotTargetDS(dsName string, node []string, filteredManifest *[]string) {
	if len(node) == 0 {
		return
	}
	nodeYAML := strings.Join(node, "\n")
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := dec.Decode([]byte(nodeYAML), nil, obj)
	if err == nil {
		if (obj.GetKind() == "DaemonSet") && (obj.GetName() == dsName) {
			return
		}
	}
	*filteredManifest = append(*filteredManifest, node...)
}

func removeControllerDaemonSetFromHelmRelease(controllerName string, dc dependency.Container) error {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	secret, err := getIngressNginxHelmReleaseSecret(k8)
	if err != nil {
		return err
	}

	jsonRelease, err := ParseReleaseSecretToJSON(secret)
	if err != nil {
		return err
	}

	manifest := jsonRelease["manifest"]
	if manifest == nil {
		return fmt.Errorf("got nil manifest from release")
	}

	var filteredManifest []string
	r := strings.NewReader(manifest.(string))
	sc := bufio.NewScanner(r)
	var node []string
	dsName := "controller-" + controllerName
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "---") && len(node) != 0 {
			appendNodeToManifestIfItNotTargetDS(dsName, node, &filteredManifest)
			node = nil
		}
		node = append(node, line)
	}
	appendNodeToManifestIfItNotTargetDS(dsName, node, &filteredManifest)
	if err := sc.Err(); err != nil {
		return fmt.Errorf("error scan manifest: %v", err)
	}

	jsonRelease["manifest"] = strings.Join(filteredManifest, "\n") + "\n"
	marshalledJSONRelease, err := json.Marshal(jsonRelease)
	if err != nil {
		return fmt.Errorf("error marshalling json release: %v", err)
	}

	compressedRelease, err := gZipData(marshalledJSONRelease)
	if err != nil {
		return fmt.Errorf("error compressing release data: %v", err)
	}
	base64Release := base64.StdEncoding.EncodeToString(compressedRelease)
	if len(base64Release) == 0 {
		return fmt.Errorf("got zero length base64 encoded release data")
	}
	secret.Data["release"] = []byte(base64Release)

	return saveIngressNginxHelmReleaseSecret(secret, k8)
}

func getIngressNginxHelmReleaseSecret(client k8s.Client) (*v1.Secret, error) {
	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"name":   "ingress-nginx",
			"owner":  "helm",
			"status": "deployed",
		},
	}

	secretList, err := client.CoreV1().Secrets("d8-system").List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(selector.MatchLabels).String()})
	if err != nil {
		return nil, err
	}
	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("no deployed `ingress-nginx` helm release found in namespace `d8-system`")
	}

	return &secretList.Items[0], nil
}

func saveIngressNginxHelmReleaseSecret(secret *v1.Secret, client k8s.Client) error {
	_, err := client.CoreV1().Secrets("d8-system").Update(context.TODO(), secret, metav1.UpdateOptions{})
	return err
}

func gUnzipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}

func gZipData(data []byte) (compressedData []byte, err error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)

	_, err = gz.Write(data)
	if err != nil {
		return
	}

	if err = gz.Flush(); err != nil {
		return
	}

	if err = gz.Close(); err != nil {
		return
	}

	compressedData = b.Bytes()

	return
}
