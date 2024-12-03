/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ensureSecretProcess func(ctx context.Context, secret *corev1.Secret, found bool) error

func ensureSecret(ctx context.Context, cli client.Client, name, namespace string, process ensureSecretProcess) (updated bool, err error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	err = cli.Get(ctx, key, &secret)
	if client.IgnoreNotFound(err) != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	secretOrig := secret.DeepCopy()
	found := err == nil

	err = process(ctx, &secret, found)
	if err != nil {
		err = fmt.Errorf("process error: %w", err)
		return
	}

	if !found {
		secret.Name = key.Name
		secret.Namespace = key.Namespace

		if err = cli.Create(ctx, &secret); err != nil {
			err = fmt.Errorf("cannot create k8s object: %w", err)
			return
		}

		updated = true
	} else {
		// Type cannot be changed, so preserve original value
		secret.Type = secretOrig.Type

		// Check than we're need to update secret
		if !reflect.DeepEqual(secretOrig, secret) {
			if err = cli.Update(ctx, &secret); err != nil {
				err = fmt.Errorf("cannot update k8s object: %w", err)
				return
			}

			if secretOrig.ResourceVersion != secret.ResourceVersion {
				updated = true
			}
		}
	}

	return
}
