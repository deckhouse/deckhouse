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

package crdmigration

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	caBundlePollInterval = 5 * time.Second
	caBundlePollTimeout  = 10 * time.Minute
)

func EnsureCRDs(ctx context.Context, c client.Client) error {
	crds, err := loadCRDs(crdDir)
	if err != nil {
		return fmt.Errorf("loading CRD manifests: %w", err)
	}

	klog.Info("waiting for CA bundle secret ", webhookSecretName)
	caBundle, err := waitForCABundle(ctx, c)
	if err != nil {
		return fmt.Errorf("waiting for CA bundle: %w", err)
	}
	klog.Info("CA bundle secret ready")

	for crdName, embeddedCRD := range crds {
		if err := ensureSingleCRD(ctx, c, crdName, embeddedCRD, caBundle); err != nil {
			return fmt.Errorf("ensuring CRD %s: %w", crdName, err)
		}
	}

	klog.Info("all CAPI CRDs ensured")
	return nil
}

func waitForCABundle(ctx context.Context, c client.Client) ([]byte, error) {
	var caBundle []byte

	err := wait.PollUntilContextTimeout(ctx, caBundlePollInterval, caBundlePollTimeout, true, func(ctx context.Context) (bool, error) {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{
			Name:      webhookSecretName,
			Namespace: capiNamespace,
		}, secret); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		ca := secret.Data["ca.crt"]
		if len(ca) == 0 {
			return false, nil
		}

		caBundle = ca
		return true, nil
	})

	return caBundle, err
}

func ensureSingleCRD(ctx context.Context, c client.Client, crdName string, embeddedCRD *apiextensionsv1.CustomResourceDefinition, caBundle []byte) error {
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err := c.Get(ctx, types.NamespacedName{Name: crdName}, existing)
	if errors.IsNotFound(err) {
		newCRD := embeddedCRD.DeepCopy()
		setMigrationSpec(newCRD, caBundle)
		if err := c.Create(ctx, newCRD); err != nil {
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("creating: %w", err)
		}
		klog.Infof("CRD %s created", crdName)
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting: %w", err)
	}

	if isMigrated(existing, caBundle) {
		klog.V(1).Infof("CRD %s already migrated", crdName)
		return nil
	}

	patch := existing.DeepCopy()
	setMigrationSpec(patch, caBundle)
	if err := c.Patch(ctx, patch, client.MergeFrom(existing)); err != nil {
		return fmt.Errorf("patching: %w", err)
	}

	klog.Infof("CRD %s migrated", crdName)
	return nil
}
