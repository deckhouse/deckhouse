/*
Copyright 2021 Flant JSC

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
	"fmt"
	"log/slog"
	"regexp"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type StorageClass struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IopsPerGB  string `json:"iopsPerGB,omitempty"`
	Iops       string `json:"iops,omitempty"`
	Throughput string `json:"throughput,omitempty"`
}

var defaultStorageClasses = []StorageClass{
	{
		Name: "gp3",
		Type: "gp3",
	},
	{
		Name: "gp2",
		Type: "gp2",
	},
	{
		Name: "sc1",
		Type: "sc1",
	},
	{
		Name: "st1",
		Type: "st1",
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "module_storageclasses",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"heritage": "deckhouse"},
			},
			FilterFunc: applyModuleStorageClassesFilter,
		},
	},
}, storageClasses)

func applyModuleStorageClassesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sc = &storagev1.StorageClass{}
	err := sdk.FromUnstructured(obj, sc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return sc, nil
}

func excludeCheck(regexps []*regexp.Regexp, storageClassName string) bool {
	for _, r := range regexps {
		if r.MatchString(storageClassName) {
			return true
		}
	}
	return false
}

func storageClasses(_ context.Context, input *go_hook.HookInput) error {
	provision := input.Values.Get("cloudProviderAws.storageClass.provision").Array()

	provisionExcludeNames := make([]gjson.Result, 0, len(provision))
	for _, sc := range provision {
		provisionExcludeNames = append(provisionExcludeNames, sc.Get("name"))
	}

	provisionRegexps := make([]*regexp.Regexp, 0, len(provisionExcludeNames))

	// compile regular expressions
	for _, excludePattern := range provisionExcludeNames {
		var r, err = regexp.Compile(excludePattern.String())
		if err != nil {
			return err
		}
		provisionRegexps = append(provisionRegexps, r)
	}

	storageClassesFilteredProvision := make([]StorageClass, 0)
	for _, storageClass := range defaultStorageClasses {
		if !excludeCheck(provisionRegexps, storageClass.Name) {
			storageClassesFilteredProvision = append(storageClassesFilteredProvision, storageClass)
		}
	}

	for _, sc := range provision {
		storageClassesFilteredProvision = append(storageClassesFilteredProvision, StorageClass{
			Name:       sc.Get("name").String(),
			Type:       sc.Get("type").String(),
			Iops:       sc.Get("iops").String(),
			IopsPerGB:  sc.Get("iopsPerGB").String(),
			Throughput: sc.Get("throughput").String(),
		})
	}

	excludeStorageClasses := input.Values.Get("cloudProviderAws.storageClass.exclude").Array()

	excludeRegexps := make([]*regexp.Regexp, 0, len(excludeStorageClasses))

	// compile regular expressions
	for _, excludePattern := range excludeStorageClasses {
		var r, err = regexp.Compile(excludePattern.String())
		if err != nil {
			return err
		}
		excludeRegexps = append(excludeRegexps, r)
	}

	storageClassesFiltered := make([]StorageClass, 0)
	for _, storageClass := range storageClassesFilteredProvision {
		if !excludeCheck(excludeRegexps, storageClass.Name) {
			storageClassesFiltered = append(storageClassesFiltered, storageClass)
		}
	}

	sort.Slice(storageClassesFiltered, func(i, j int) bool {
		return storageClassesFiltered[i].Name < storageClassesFiltered[j].Name
	})

	if len(storageClassesFiltered) != 0 {
		input.Values.Set("cloudProviderAws.internal.storageClasses", storageClassesFiltered)
	} else {
		input.Values.Set("cloudProviderAws.internal.storageClasses", []StorageClass{})
	}

	rawSCs, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "module_storageclasses")
	if err != nil {
		return fmt.Errorf("unmarshal snapshot module_storageclasses: %w", err)
	}

	existedStorageClasses := make([]StorageClass, 0, len(rawSCs))
	for _, sc := range rawSCs {
		existedStorageClasses = append(existedStorageClasses, StorageClass{
			Name:       sc.Name,
			Type:       sc.Parameters["type"],
			Iops:       sc.Parameters["iops"],
			IopsPerGB:  sc.Parameters["iopsPerGB"],
			Throughput: sc.Parameters["throughput"],
		})
	}

	for _, sc := range existedStorageClasses {
		if !isModified(storageClassesFiltered, sc) {
			continue
		}
		input.Logger.Info("Deleting storageclass because its parameters has been changed", slog.String("storage_class", sc.Name))
		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", sc.Name)
	}

	return nil
}

func isModified(storageClasses []StorageClass, storageClass StorageClass) bool {
	for _, sc := range storageClasses {
		if sc.Name == storageClass.Name && sc != storageClass {
			return true
		}
	}
	return false
}
