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

package testenv

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

const (
	nodeGroupCRDName = "nodegroups.deckhouse.io"
	crdSchemaTimeout = 10 * time.Second
	crdSchemaPoll    = 100 * time.Millisecond
)

func CreateNodeGroup(ctx context.Context, c client.Client, ng *deckhousev1.NodeGroup) error {
	if ng.Spec.SystemType == deckhousev1.SystemTypeLegacyMutable || ng.Spec.SystemType == deckhousev1.SystemTypeLegacyImmutable {
		return CreateLegacyNodeGroup(ctx, c, ng)
	}
	return c.Create(ctx, ng)
}

func CreateLegacyNodeGroup(ctx context.Context, c client.Client, ng client.Object) error {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"})
	if err := c.Get(ctx, client.ObjectKey{Name: nodeGroupCRDName}, crd); err != nil {
		return fmt.Errorf("get the NodeGroup CRD: %w", err)
	}
	shipped, _, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil {
		return fmt.Errorf("read the NodeGroup CRD versions: %w", err)
	}

	if err := setCRDVersions(ctx, c, crd, withLegacySystemTypes(shipped)); err != nil {
		return err
	}
	createErr := wait.PollUntilContextTimeout(ctx, crdSchemaPoll, crdSchemaTimeout, true, func(ctx context.Context) (bool, error) {
		err := c.Create(ctx, ng)
		if apierrors.IsInvalid(err) {
			return false, nil
		}
		return err == nil, err
	})
	if err := setCRDVersions(ctx, c, crd, shipped); err != nil {
		return err
	}
	if createErr != nil {
		return fmt.Errorf("create %s while the CRD takes the legacy spelling: %w", ng.GetName(), createErr)
	}
	return waitUntilLegacyRefused(ctx, c)
}

func setCRDVersions(ctx context.Context, c client.Client, crd *unstructured.Unstructured, versions []any) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := c.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
			return err
		}
		if err := unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions"); err != nil {
			return err
		}
		return c.Update(ctx, crd)
	})
}

func withLegacySystemTypes(versions []any) []any {
	legacy := make([]any, 0, len(versions))
	for _, version := range versions {
		version := runtime.DeepCopyJSON(version.(map[string]any))
		path := []string{"schema", "openAPIV3Schema", "properties", "spec", "properties", "systemType", "enum"}
		enum, _, _ := unstructured.NestedSlice(version, path...)
		_ = unstructured.SetNestedSlice(version, append(enum, "Mutable", "Immutable"), path...)
		legacy = append(legacy, version)
	}
	return legacy
}

func waitUntilLegacyRefused(ctx context.Context, c client.Client) error {
	probe := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": "legacy-systemtype-probe"},
		"spec":       map[string]any{"nodeType": "Static", "systemType": "Immutable"},
	}}
	err := wait.PollUntilContextTimeout(ctx, crdSchemaPoll, crdSchemaTimeout, true, func(ctx context.Context) (bool, error) {
		return apierrors.IsInvalid(c.Create(ctx, probe.DeepCopy(), client.DryRunAll)), nil
	})
	if err != nil {
		return fmt.Errorf("the NodeGroup CRD still takes the legacy spelling: %w", err)
	}
	return nil
}
