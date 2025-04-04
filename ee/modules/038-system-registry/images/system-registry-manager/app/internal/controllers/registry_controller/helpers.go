/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ensureSecretProcess func(ctx context.Context, secret *corev1.Secret, found bool) error

func ensureSecret(ctx context.Context, cli client.Client, name, namespace string, process ensureSecretProcess) (bool, error) {
	var (
		updated bool
		err     error
	)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	err = cli.Get(ctx, key, &secret)
	if client.IgnoreNotFound(err) != nil {
		err = fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
		return updated, err
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
		return updated, err
	}

	if !found {
		secret.Name = key.Name
		secret.Namespace = key.Namespace

		if err = cli.Create(ctx, &secret); err != nil {
			err = fmt.Errorf("cannot create k8s object: %w", err)
			return updated, err
		}

		updated = true
	} else {
		// Type cannot be changed, so preserve original value
		secret.Type = secretOrig.Type

		// Check than we're need to update secret
		if !reflect.DeepEqual(secretOrig, secret) {
			if err = cli.Update(ctx, &secret); err != nil {
				err = fmt.Errorf("cannot update k8s object: %w", err)
				return updated, err
			}

			if secretOrig.ResourceVersion != secret.ResourceVersion {
				updated = true
			}
		}
	}

	return updated, err
}

// getRegistryAddressAndPathFromImagesRepo returns the registry address and path from the given image repository.
func getRegistryAddressAndPathFromImagesRepo(imgRepo string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}
