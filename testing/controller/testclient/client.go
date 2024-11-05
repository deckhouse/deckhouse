// Copyright 2024 Flant JSC
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

package testclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/crds"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// TODO: move schemaBuilder to separate package
var schemaBuilder = runtime.NewSchemeBuilder(
	v1alpha1.AddToScheme,
	coordv1.AddToScheme,
	appsv1.AddToScheme,
	corev1.AddToScheme,
	apiextensionsv1.AddToScheme,
)

// TODO: implement StatusClient and SubResourceClientConstructor
var _ client.Client = (*Client)(nil)

func New(logger *log.Logger, initObjects []client.Object) (*Client, error) {
	sc := runtime.NewScheme()
	err := schemaBuilder.AddToScheme(sc)
	if err != nil {
		return nil, fmt.Errorf("build scheme: %w", err)
	}

	CRDs, err := crds.List()
	if err != nil {
		return nil, fmt.Errorf("list crds: %w", err)
	}

	openAPISchema := make(map[string]*spec.Schema)
	crdsObjects := make([]client.Object, 0, len(CRDs))
	validators := make(map[schema.GroupVersionKind]validation.SchemaValidator, len(CRDs))
	for _, crd := range CRDs {
		crdsObjects = append(crdsObjects, &crd)

		err = addValidator(crd, validators, openAPISchema)
		if err != nil {
			return nil, fmt.Errorf("add validator: %w", err)
		}
	}

	cl := fake.NewClientBuilder().
		WithScheme(sc).
		WithObjects(crdsObjects...).
		WithObjects(initObjects...).
		WithStatusSubresource(
			&v1alpha1.ModuleSource{},
			&v1alpha1.ModuleRelease{},
		).
		Build()

	return &Client{logger: logger, Client: cl, validators: validators, openAPISchema: openAPISchema}, nil
}

type Client struct {
	logger *log.Logger
	client.Client
	validators    map[schema.GroupVersionKind]validation.SchemaValidator
	openAPISchema map[string]*spec.Schema
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	k := obj.GetObjectKind()
	var v validation.SchemaValidator
	if k != nil {
		v = c.validators[k.GroupVersionKind()]
	}

	if v != nil {
		result := v.Validate(obj)

		for _, warn := range result.Warnings {
			c.logger.Warn(warn.Error())
		}

		if len(result.Errors) > 0 {
			return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
		}
	}

	return c.Client.Create(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	k := obj.GetObjectKind()
	var v validation.SchemaValidator
	if k != nil {
		// TODO: 	v = c.validators[k.GroupVersionKind()]
	}

	if v != nil {
		result := v.ValidateUpdate(obj, nil)

		for _, warn := range result.Warnings {
			c.logger.Warn(warn.Error())
		}

		if len(result.Errors) > 0 {
			return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
		}
	}

	return c.Client.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, object client.Object, p client.Patch, opts ...client.PatchOption) error {
	k := object.GetObjectKind()
	var v validation.SchemaValidator
	if k != nil {
		// TODO: v = c.validators[k.GroupVersionKind()]
	}

	if v == nil {
		return c.Client.Patch(ctx, object, p, opts...)
	}

	rawPatch, err := p.Data(object)
	if err != nil {
		return fmt.Errorf("generate patch: %w", err)
	}

	err = c.Client.Patch(ctx, object, p, opts...)
	if err != nil {
		return fmt.Errorf("patch: %w", err)
	}

	var tmp any
	err = json.Unmarshal(rawPatch, &tmp)
	if err != nil {
		return fmt.Errorf("unmarshal raw patch: %w", err)
	}

	c.logger.Debug("validating patch: ", "patch", tmp)

	patched, err := patch(object, p.Type(), rawPatch, c.Scheme(), c.openAPISchema)
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}

	result := v.ValidateUpdate(patched, object)
	for _, warn := range result.Warnings {
		c.logger.Warn(warn.Error())
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
	}

	return nil
}
