/*
Copyright 2024 Flant JSC

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

package migrate

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	flowSchemaGroup = "flowcontrol.apiserver.k8s.io"
	helmAnnotation  = "meta.helm.sh/release-name"
)

// TODO: remove after the release 1.69
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(changeAnnotation))

func flowSchemaAPIVersion(kubeVersion *semver.Version) string {
	Kubernetes126 := semver.MustParse("1.26")
	Kubernetes129 := semver.MustParse("1.29")
	switch {
	case kubeVersion.GreaterThan(Kubernetes129):
		return "v1"
	case kubeVersion.GreaterThan(Kubernetes126):
		return "v1beta3"
	default:
		return "v1beta2"
	}
}

func changeAnnotationUnstructured(
	ctx context.Context,
	log go_hook.Logger,
	schema schema.GroupVersionResource,
	dynamicClient dynamic.Interface,
) error {
	labelSelector := v1.ListOptions{
		LabelSelector: "heritage=deckhouse",
	}
	log.Infof("started migration for gvr: %s/%s %s", schema.Group, schema.Version, schema.Resource)
	client := dynamicClient.Resource(schema)

	list, err := client.List(ctx, labelSelector)
	if err != nil {
		return err
	}
	log.Infof("sucessfully got resources from cluster: %+v", list)

	for _, v := range list.Items {
		obj := v

		annotations, found, err := unstructured.NestedStringMap(obj.Object, "metadata", "annotations")
		if err != nil {
			return err
		}
		if !found {
			log.Infof("annotations not found for resource %s", obj.GetName())
			continue
		}

		if annotations[helmAnnotation] == "deckhouse" {
			log.Infof("annotation %s is equal %s", helmAnnotation, obj.GetName())
			continue
		}

		annotations[helmAnnotation] = "deckhouse"

		if err := unstructured.SetNestedStringMap(obj.Object, annotations, "metadata", "annotations"); err != nil {
			return err
		}
		log.Infof("successfully set %s to %s in %s", helmAnnotation, "deckhouse", obj.GetName())

		if _, err := client.Update(ctx, &obj, v1.UpdateOptions{}); err != nil {
			return err
		}
		log.Infof("successfully updated object %s", obj.GetName())
	}

	return nil
}

func changeAnnotation(input *go_hook.HookInput, dc dependency.Container) error {
	log := input.Logger

	ctx := context.Background()
	val := input.Values.Get("global.discovery.kubernetesVersion").String()
	log.Infof("Successfully got kubernetesVersion, value is %s", val)
	ver, err := semver.NewVersion(val)
	if err != nil {
		return fmt.Errorf("global.discovery.kubernetesVersion contains a malformed semver: %s: %w", val, err)
	}
	log.Infof("Successfully parsed kubernetesVersion to semver", val)

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	schemas := []schema.GroupVersionResource{
		{
			Group:    "scheduling.k8s.io",
			Version:  "v1",
			Resource: "priorityclasses",
		},
		{
			Group:    flowSchemaGroup,
			Version:  flowSchemaAPIVersion(ver),
			Resource: "flowschemas",
		},
		{
			Group:    flowSchemaGroup,
			Version:  flowSchemaAPIVersion(ver),
			Resource: "prioritylevelconfigurations",
		},
	}

	for _, schema := range schemas {
		if err := changeAnnotationUnstructured(ctx, log, schema, k8sClient.Dynamic()); err != nil {
			return err
		}
	}

	return nil
}
