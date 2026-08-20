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
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type StorageClass struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	BlockSize string `json:"blockSize,omitempty"`
}

var defaultStorageClasses = []StorageClass{
	{
		Name: "network-hdd",
		Type: "network-hdd",
	},
	{
		Name: "network-ssd",
		Type: "network-ssd",
	},
	{
		Name: "network-ssd-nonreplicated",
		Type: "network-ssd-nonreplicated",
	},
	{
		Name: "network-ssd-io-m3",
		Type: "network-ssd-io-m3",
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

func compileRegexps(patterns []string) ([]*regexp.Regexp, error) {
	regexps := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, r)
	}

	return regexps, nil
}

func matchCheck(regexps []*regexp.Regexp, storageClassName string) bool {
	for _, r := range regexps {
		if r.MatchString(storageClassName) {
			return true
		}
	}
	return false
}

func storageClasses(_ context.Context, input *go_hook.HookInput) error {
	provisionValues := input.Values.Get("cloudProviderYandex.storageClass.provision").Array()

	provision := make([]StorageClass, 0, len(provisionValues))
	provisionNames := make([]string, 0, len(provisionValues))
	for _, sc := range provisionValues {
		provision = append(provision, StorageClass{
			Name:      sc.Get("name").String(),
			Type:      sc.Get("type").String(),
			BlockSize: sc.Get("blockSize").String(),
		})
		provisionNames = append(provisionNames, sc.Get("name").String())
	}

	// StorageClasses defined in `provision` override the ones created by default
	provisionRegexps, err := compileRegexps(provisionNames)
	if err != nil {
		return fmt.Errorf("storageClass.provision names compilation error: %v", err)
	}

	storageClassesFilteredProvision := make([]StorageClass, 0, len(defaultStorageClasses)+len(provision))
	for _, storageClass := range defaultStorageClasses {
		if !matchCheck(provisionRegexps, storageClass.Name) {
			storageClassesFilteredProvision = append(storageClassesFilteredProvision, storageClass)
		}
	}
	storageClassesFilteredProvision = append(storageClassesFilteredProvision, provision...)

	excludeValues := input.Values.Get("cloudProviderYandex.storageClass.exclude").Array()

	excludePatterns := make([]string, 0, len(excludeValues))
	for _, excludePattern := range excludeValues {
		excludePatterns = append(excludePatterns, excludePattern.String())
	}

	excludeRegexps, err := compileRegexps(excludePatterns)
	if err != nil {
		return fmt.Errorf("storageClass.exclude set creation error: %v", err)
	}

	storageClassesFiltered := make([]StorageClass, 0, len(storageClassesFilteredProvision))
	for _, storageClass := range storageClassesFilteredProvision {
		if !matchCheck(excludeRegexps, storageClass.Name) {
			storageClassesFiltered = append(storageClassesFiltered, storageClass)
		}
	}

	sort.Slice(storageClassesFiltered, func(i, j int) bool {
		return storageClassesFiltered[i].Name < storageClassesFiltered[j].Name
	})

	input.Values.Set("cloudProviderYandex.internal.storageClasses", storageClassesFiltered)

	// StorageClass parameters are immutable, so a StorageClass whose parameters were
	// changed in the module configuration has to be recreated
	rawSCs, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "module_storageclasses")
	if err != nil {
		return fmt.Errorf("unmarshal snapshot module_storageclasses: %w", err)
	}

	for _, sc := range rawSCs {
		existedStorageClass := StorageClass{
			Name:      sc.Name,
			Type:      sc.Parameters["typeID"],
			BlockSize: sc.Parameters["blockSize"],
		}

		if !isModified(storageClassesFiltered, existedStorageClass) {
			continue
		}

		input.Logger.Info("Deleting storageclass because its parameters has been changed", slog.String("storage_class", existedStorageClass.Name))
		input.PatchCollector.Delete("storage.k8s.io/v1", "StorageClass", "", existedStorageClass.Name)
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
