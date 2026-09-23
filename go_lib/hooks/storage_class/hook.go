/*
Copyright 2026 Flant JSC

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

// Package storage_class registers a hook that computes the StorageClasses a module publishes.
//
// The hook runs a pipeline the module assembles from steps over entries of its own type T, which only has
// to implement Namer (PruneModified also needs T to be comparable): sources fill the desired set,
// transformations reshape it, and sinks publish it to values and reconcile the cluster. Every step
// works on one State, so a module can put the library steps in any order and add its own.
//
//	func (class StorageClass) GetName() string { return class.Name }
//
//	var _ = storage_class.RegisterHook(
//		storage_class.Config{Order: 20, ModuleName: "cloud-provider-yandex"},
//		storage_class.Append(storage_class.Static(defaults...)),
//		storage_class.OverrideByName(storage_class.FromValues[StorageClass]("cloudProviderYandex.storage.parameters.provisionedStorageClasses")),
//		storage_class.Exclude[StorageClass]("cloudProviderYandex.storage.parameters.excludedStorageClasses"),
//		storage_class.SortByName[StorageClass](),
//		storage_class.Publish[StorageClass]("cloudProviderYandex.internal.storageClasses"),
//		storage_class.PruneModified(fromObject),
//	)
package storage_class

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var (
	// ErrStop ends the pipeline early without failing the hook: the steps after the one that returned it
	// do not run, and nothing they would have published is touched.
	ErrStop = errors.New("stop storage class pipeline")
)

// Namer is a desired StorageClass entry: anything that knows the name of the StorageClass it stands for.
type Namer interface {
	GetName() string
}

// Config places the hook and scopes the StorageClasses it sees.
type Config struct {
	// Order is the OnBeforeHelm order of the hook.
	Order float64
	// ModuleName scopes the snapshot to the StorageClasses the module renders: the ones labelled
	// heritage=deckhouse and module=<ModuleName>, as helm_lib_module_labels does.
	ModuleName string
}

// State is what the steps of one run share.
type State[T Namer] struct {
	Input      *go_hook.HookInput
	ModuleName string

	// Actual are the module's StorageClasses in the cluster.
	Actual []storagev1.StorageClass
	// Desired is the set the steps build and reshape; sinks publish it.
	Desired []T
}

// Step is one stage of the pipeline. It may return ErrStop to end the run early.
type Step[T Namer] func(ctx context.Context, state *State[T]) error

// Source produces entries for a step that fills the desired set, such as Append or OverrideByName.
type Source[T Namer] func(ctx context.Context, state *State[T]) ([]T, error)

// RegisterHook registers an OnBeforeHelm hook that runs the steps in order on every run.
func RegisterHook[T Namer](cfg Config, steps ...Step[T]) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: cfg.Order},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "storage_classes",
				ApiVersion: "storage.k8s.io/v1",
				Kind:       "StorageClass",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"heritage": "deckhouse",
						"module":   cfg.ModuleName,
					},
				},
				FilterFunc: filterStorageClass,
			},
		},
	}, handleStorageClassFunc(cfg, steps))
}

func filterStorageClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	storageClass := &storagev1.StorageClass{}
	if err := sdk.FromUnstructured(obj, storageClass); err != nil {
		return nil, fmt.Errorf("convert StorageClass: %w", err)
	}

	return storageClass, nil
}

func handleStorageClassFunc[T Namer](cfg Config, steps []Step[T]) func(context.Context, *go_hook.HookInput) error {
	return func(ctx context.Context, input *go_hook.HookInput) error {
		actual, err := sdkobjectpatch.UnmarshalToStruct[storagev1.StorageClass](input.Snapshots, "storage_classes")
		if err != nil {
			return fmt.Errorf("unmarshal storage_classes snapshot: %w", err)
		}

		state := &State[T]{
			Input:      input,
			ModuleName: cfg.ModuleName,
			Actual:     actual,
		}

		return Run(ctx, state, steps...)
	}
}

// Run executes the steps on the state in order, treating ErrStop as a successful early end.
func Run[T Namer](ctx context.Context, state *State[T], steps ...Step[T]) error {
	for _, step := range steps {
		if err := step(ctx, state); err != nil {
			if errors.Is(err, ErrStop) {
				return nil
			}

			return err
		}
	}

	return nil
}
