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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var getNodeGroups = &go_hook.HookConfig{
	Queue:       "/modules/node-manager",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}

var _ = sdk.RegisterFunc(getNodeGroups, dependency.WithExternalDependencies(annotateNodeGroups))

func annotateNodeGroups(input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}
	ngInterface := kubeClient.Dynamic().Resource(schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}).Namespace("")
	ngObjs, err := ngInterface.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ngObj := range ngObjs.Items {
		var nodeGroup ngv1.NodeGroup
		err := sdk.FromUnstructured(&ngObj, &nodeGroup)
		if err != nil {
			return err
		}
		ng := NodeGroupCrdInfo{
			Name:            nodeGroup.GetName(),
			Spec:            nodeGroup.Spec,
			ManualRolloutID: nodeGroup.GetAnnotations()["manual-rollout-id"],
		}
		ngBytes, err := json.Marshal(ng)
		if err != nil {
			return fmt.Errorf("cannot marshal node group object: %v", err)
		}
		checkSum := utils_checksum.CalculateChecksum(string(ngBytes))

		noticedAnnotation, found, err := unstructured.NestedString(ngObj.Object, "metadata", "annotations", "deckhouse.io/node-manager-hook-noticed")
		if err != nil {
			return fmt.Errorf("cannot get node group object annotation: %v", err)
		}

		if !found || !checksumEqualsAnnotation(checkSum, noticedAnnotation) {
			if err := unstructured.SetNestedField(ngObj.Object, "False", "metadata", "annotations", "deckhouse.io/node-manager-hook-synced"); err != nil {
				return fmt.Errorf("cannot set node group object annotation: %v", err)
			}
		} else {
			if err := unstructured.SetNestedField(ngObj.Object, "True", "metadata", "annotations", "deckhouse.io/node-manager-hook-synced"); err != nil {
				return fmt.Errorf("cannot set node group object annotation: %v", err)
			}
		}

		if err := unstructured.SetNestedField(ngObj.Object, fmt.Sprintf("%s/%s", time.Now().Format(time.RFC3339), checkSum), "metadata", "annotations", "deckhouse.io/node-manager-hook-processed"); err != nil {
			return fmt.Errorf("cannot set node group object annotation: %v", err)
		}
		_, err = ngInterface.Update(context.TODO(), &ngObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("cannot update node group object: %v", err)
		}
	}

	return nil
}
