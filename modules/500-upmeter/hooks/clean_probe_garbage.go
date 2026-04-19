// Copyright 2022 Flant JSC
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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// This hook deletes abandoned objects produced by upmeter.
//
// TODO (shvgn): Change this hook in Deckhouse v1.35, so it would track objects created by agents
// that are not present anymore, e.g. when multi-master was changed to single-master.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/upmeter/self_cleaning",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "delete_probe_garbage",
			Crontab: "*/15 * * * *",
		},
	},
}, dependency.WithExternalDependencies(
	func(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
		k := dc.MustGetK8sClient()
		ctx := context.TODO()

		repos := []objectRepository{
			&configMapRepo{k},
			&certRepo{k},
			&certSecretRepo{k},
			&deployRepo{k},
			&podRepo{k},
			&namespaceRepo{k},
			&upmeterHookProbeRepo{k},
		}

		for _, r := range repos {
			if err := cleanGarbage(ctx, r); err != nil {
				// The queue shouldn't be stopped event if there is an API error
				input.Logger.Warn("clean garbage", log.Err(err))
			}
		}

		return nil
	},
))

func cleanGarbage(ctx context.Context, repo objectRepository) error {
	objects, err := repo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing: %v", err)
	}

	limit := 10 // being gentle
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	for _, obj := range objects {
		// An object should be older than probe run interval, 5 min is safe
		isOldEnough := obj.GetCreationTimestamp().Time.Before(fiveMinAgo)
		if !isOldEnough {
			continue
		}
		if err := repo.Delete(ctx, obj.GetName()); err != nil {
			return fmt.Errorf("deleting %s: %v", obj.GetName(), err)
		}
		limit--
	}

	return nil
}

type objectRepository interface {
	// List returns abstract object as a container of the name and the creation timestamp
	List(context.Context) ([]metav1.Object, error)

	// Delete works with objects by name on individual basis
	Delete(context.Context, string) error
}

type configMapRepo struct {
	k k8s.Client
}

func (r *configMapRepo) List(ctx context.Context) ([]metav1.Object, error) {
	list, err := r.k.CoreV1().
		ConfigMaps("d8-upmeter").
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, list.Items[i].GetObjectMeta())
	}
	return objects, nil
}

func (r *configMapRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().ConfigMaps("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

var certificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

type certRepo struct {
	k k8s.Client
}

func (r *certRepo) List(ctx context.Context) ([]metav1.Object, error) {
	list, err := r.k.Dynamic().
		Resource(certificateGVR).
		Namespace("d8-upmeter").
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		// This response depends on the presence of cert-manager certificate CRD
		emptyList := make([]metav1.Object, 0)
		return emptyList, nil
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, &list.Items[i])
	}
	return objects, nil
}

func (r *certRepo) Delete(ctx context.Context, name string) error {
	return r.k.Dynamic().
		Resource(certificateGVR).
		Namespace("d8-upmeter").
		Delete(ctx, name, metav1.DeleteOptions{})
}

type certSecretRepo struct {
	k k8s.Client
}

func (r *certSecretRepo) List(ctx context.Context) ([]metav1.Object, error) {
	// Cert secrets don't have the 'heritage=upmeter' label, we have to filter them by name mask
	list, err := r.k.CoreV1().
		Secrets("d8-upmeter").
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		secret := list.Items[i]
		if !strings.HasPrefix(secret.GetName(), "upmeter-cm-probe") {
			continue
		}
		objects = append(objects, secret.GetObjectMeta())
	}
	return objects, nil
}

func (r *certSecretRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().
		Secrets("d8-upmeter").
		Delete(ctx, name, metav1.DeleteOptions{})
}

type namespaceRepo struct {
	k k8s.Client
}

func (r *namespaceRepo) List(ctx context.Context) ([]metav1.Object, error) {
	list, err := r.k.CoreV1().
		Namespaces().
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, list.Items[i].GetObjectMeta())
	}
	return objects, nil
}

func (r *namespaceRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}

type podRepo struct {
	k k8s.Client
}

func (r *podRepo) List(ctx context.Context) ([]metav1.Object, error) {
	list, err := r.k.CoreV1().
		Pods("d8-upmeter").
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, list.Items[i].GetObjectMeta())
	}
	return objects, nil
}

func (r *podRepo) Delete(ctx context.Context, name string) error {
	return r.k.CoreV1().Pods("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

type deployRepo struct {
	k k8s.Client
}

func (r *deployRepo) List(ctx context.Context) ([]metav1.Object, error) {
	list, err := r.k.AppsV1().
		Deployments("d8-upmeter").
		List(ctx, metav1.ListOptions{LabelSelector: "heritage=upmeter"})
	if err != nil {
		return nil, err
	}
	objects := make([]metav1.Object, 0, len(list.Items))
	for i := range list.Items {
		objects = append(objects, list.Items[i].GetObjectMeta())
	}
	return objects, nil
}

func (r *deployRepo) Delete(ctx context.Context, name string) error {
	return r.k.AppsV1().Deployments("d8-upmeter").Delete(ctx, name, metav1.DeleteOptions{})
}

var upmeterHookProbeGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "upmeterhookprobes",
}

type upmeterHookProbeRepo struct {
	k k8s.Client
}

func (r *upmeterHookProbeRepo) List(_ context.Context) ([]metav1.Object, error) {
	obj := &unstructured.Unstructured{}
	obj.SetName("35d78cbb") // empty NODE_NAME results in this hash, fixing the bug
	obj.SetCreationTimestamp(metav1.NewTime(time.Now().Add(-1 * time.Hour)))
	return []metav1.Object{obj}, nil
}

func (r *upmeterHookProbeRepo) Delete(ctx context.Context, name string) error {
	err := r.k.Dynamic().
		Resource(upmeterHookProbeGVR).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err == nil || apierrors.IsNotFound(err) {
		// Since we look for a specific name, it only deletes once
		return nil
	}
	return err
}
