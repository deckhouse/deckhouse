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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationerrors "k8s.io/kube-openapi/pkg/validation/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/deckhouse/deckhouse-controller/crds"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/project"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TODO: implement StatusClient and SubResourceClientConstructor
var _ client.Client = (*Client)(nil)

func New(logger *log.Logger, initObjects []client.Object) (*Client, error) {
	sc, err := project.Scheme()
	if err != nil {
		return nil, fmt.Errorf("build scheme: %w", err)
	}

	CRDs, err := crds.List()
	if err != nil {
		return nil, fmt.Errorf("list crds: %w", err)
	}

	crdsObjects := make([]client.Object, 0, len(CRDs))
	validators := make(map[schema.GroupVersionKind]validation.SchemaValidator, len(CRDs))
	for _, crd := range CRDs {
		crdsObjects = append(crdsObjects, &crd)

		err = addValidator(crd, validators)
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
			&v1alpha1.Module{},
		).
		Build()

	validator := NewValidator(logger.Named("custom resource schema validator"), validators)
	return &Client{logger: logger, Client: cl, validator: validator}, nil
}

type Client struct {
	logger *log.Logger
	client.Client
	validator *Validator
}

func (c *Client) Validator() *Validator {
	return c.validator
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	result := c.validator.Validate(obj)
	if result != nil {
		for _, warn := range result.Warnings {
			c.logger.Warn(warn.Error())
		}

		result.Errors = ignoreStatus(result.Errors)
		if len(result.Errors) > 0 {
			return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
		}
	}

	return c.Client.Create(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	result := c.validator.ValidateUpdate(obj, nil)
	if result != nil {
		for _, warn := range result.Warnings {
			c.logger.Warn(warn.Error())
		}

		result.Errors = ignoreStatus(result.Errors)
		if len(result.Errors) > 0 {
			return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
		}
	}

	return c.Client.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
	rawPatch, err := p.Data(obj)
	if err != nil {
		return fmt.Errorf("generate patch: %w", err)
	}

	newObj, err := patch(obj, p.Type(), rawPatch, c.Scheme())
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}

	result := c.validator.ValidateUpdate(newObj, obj)
	if result != nil {
		for _, warn := range result.Warnings {
			c.logger.Warn(warn.Error())
		}

		result.Errors = ignoreStatus(result.Errors)
		if len(result.Errors) > 0 {
			return fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...))
		}
	}

	return c.Client.Patch(ctx, obj, p, opts...)
}

func ignoreStatus(errs []error) []error {
	result := make([]error, 0, len(errs))

	for _, err := range errs {
		var vErr *validationerrors.Validation
		ok := errors.As(err, &vErr)
		if ok && strings.HasPrefix(vErr.Name, "status.") {
			continue
		}

		result = append(result, err)
	}

	return result
}
