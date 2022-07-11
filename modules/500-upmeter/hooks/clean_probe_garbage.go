// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

// migration: Delete redundant objects
//
// TODO (shvgn): Delete this hook in Deckhouse v1.35
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "delete_probe_garbage",
			Crontab: "33 * * * *",
		},
	},
}, dependency.WithExternalDependencies(
	func(input *go_hook.HookInput, dc dependency.Container) error {
		k := dc.MustGetK8sClient()
		ctx := context.TODO()

		repos := []nameRepo{
			&configMapRepo{k},
			&certRepo{k},
			&deployRepo{k},
			&podRepo{k},
			&namespaceRepo{k},
		}

		for _, r := range repos {
			if err := cleanGarbage(ctx, r); err != nil {
				return err
			}
		}

		return nil
	},
))

func cleanGarbage(ctx context.Context, repo nameRepo) error {
	names, err := repo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing: %v", err)
	}

	limit := 10 // being gentle
	for _, name := range names {
		if err := repo.Delete(ctx, name); err != nil {
			return fmt.Errorf("deleting %q: %v", name, err)
		}
		limit--
	}

	return nil
}

type nameRepo interface {
	List(context.Context) ([]string, error)
	Delete(context.Context, string) error
}

func isOldEnough(creationTimestamp time.Time) bool {
	return creationTimestamp.Before(time.Now().Add(-5 * time.Minute))
}

var certificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

type configMapRepo struct {
	k k8s.Client
}

func (r *configMapRepo) List(ctx context.Context) ([]string, error) {
	list, err := r.k.CoreV1().ConfigMaps("d8-upmeter").List(ctx, metav1.ListOptions{
		LabelSelector: "heritage=upmeter",
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, x := range list.Items {
		if isOldEnough(x.GetCreationTimestamp().Time) {
			names = append(names, x.GetName())
		}
	}
	return names, nil
}

func (r *configMapRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().ConfigMaps("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

type certRepo struct {
	k k8s.Client
}

func (r *certRepo) List(ctx context.Context) ([]string, error) {
	list, err := r.k.Dynamic().Resource(certificateGVR).Namespace("d8-upmeter").
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, x := range list.Items {
		if isOldEnough(x.GetCreationTimestamp().Time) {
			names = append(names, x.GetName())
		}
	}
	return names, nil
}

func (r *certRepo) Delete(ctx context.Context, name string) error {
	return r.k.Dynamic().Resource(certificateGVR).Namespace("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

type namespaceRepo struct {
	k k8s.Client
}

func (r *namespaceRepo) List(ctx context.Context) ([]string, error) {
	list, err := r.k.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, x := range list.Items {
		if isOldEnough(x.GetCreationTimestamp().Time) {
			names = append(names, x.GetName())
		}
	}
	return names, nil
}

func (r *namespaceRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}

type podRepo struct {
	k k8s.Client
}

func (r *podRepo) List(ctx context.Context) ([]string, error) {
	list, err := r.k.CoreV1().Pods("d8-upmeter").List(ctx, metav1.ListOptions{
		LabelSelector: "heritage=upmeter",
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, x := range list.Items {
		if isOldEnough(x.GetCreationTimestamp().Time) {
			names = append(names, x.GetName())
		}
	}
	return names, nil
}

func (r *podRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().Pods("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

type deployRepo struct {
	k k8s.Client
}

func (r *deployRepo) List(ctx context.Context) ([]string, error) {
	list, err := r.k.AppsV1().Deployments("d8-upmeter").List(ctx, metav1.ListOptions{
		LabelSelector: "heritage=upmeter",
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, x := range list.Items {
		if isOldEnough(x.GetCreationTimestamp().Time) {
			names = append(names, x.GetName())
		}
	}
	return names, nil
}

func (r *deployRepo) Delete(ctx context.Context, name string) error {
	return r.k.AppsV1().Deployments("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}
