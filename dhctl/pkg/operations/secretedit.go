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

package operations

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	dh_config "github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const allowUnsafeAnnotation = "deckhouse.io/allow-unsafe"

// editFunc allows tests to swap the editor with a deterministic mock without
// reaching for package-level state (see secretedit_test.go).
type editFunc func(context.Context, []byte, *options.GlobalOptions, EditOptions) ([]byte, error)

var abstractEditing editFunc = Edit

var emptySecret = &v1.Secret{
	TypeMeta: metav1.TypeMeta{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "Secret",
	},
	ObjectMeta: metav1.ObjectMeta{},
	Type:       v1.SecretTypeOpaque,
	Data:       make(map[string][]byte),
}

func SecretEdit(
	ctx context.Context,
	kubeCl *client.KubernetesClient, name, namespace, secret, dataKey string,
	labels map[string]string,
	globalOptions *options.GlobalOptions,
	editOpts EditOptions,
) error {
	config, err := kubeCl.CoreV1().Secrets(namespace).Get(ctx, secret, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Secret %s in namespace %s was not found, and will be created", secret, namespace))
		config = emptySecret.DeepCopy()
		config.ObjectMeta.Name, config.ObjectMeta.Namespace = secret, namespace
	case err != nil:
		return err
	}

	for k, v := range labels {
		if config.ObjectMeta.Labels == nil {
			config.ObjectMeta.Labels = make(map[string]string, len(labels))
		}

		config.ObjectMeta.Labels[k] = v
	}

	configData := config.Data[dataKey]

	var modifiedData []byte
	err = dh_config.PrepareCandiDir(ctx, kubeCl, globalOptions)
	if err != nil {
		return err
	}
	tomb.WithoutInterruptions(func() { modifiedData, err = abstractEditing(ctx, configData, globalOptions, editOpts) })
	if err != nil {
		return err
	}

	// This flag is validating by webhooks to allow editing unsafe resource's fields.
	if editOpts.SanityCheck {
		addUnsafeAnnotation(config)
	}

	return dhlog.RunProcess(
		ctx,
		dhlog.FromContext(ctx),
		fmt.Sprintf("Save %s to the Kubernetes cluster", name),
		func(ctx context.Context) error {
			if string(configData) == string(modifiedData) {
				dhlog.FromContext(ctx).InfoContext(ctx, "Configurations are equal. Nothing to update.")
				return nil
			}

			config.Data[dataKey] = modifiedData

			return retry.
				NewLoop(fmt.Sprintf("Apply %s secret", secret), 5, 5*time.Second).
				Run(func() error {
					_, err = kubeCl.CoreV1().Secrets(namespace).Update(ctx, config, metav1.UpdateOptions{})
					switch {
					case errors.IsNotFound(err):
						dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Creating new Secret %s in namespace %s", secret, namespace))
						if _, err = kubeCl.CoreV1().Secrets(namespace).Create(ctx, config, metav1.CreateOptions{}); err != nil {
							return err
						}
					case err != nil:
						return err
					}

					if editOpts.SanityCheck {
						dhlog.FromContext(ctx).InfoContext(ctx, "Removing allow-unsafe annotation")
						removeUnsafeAnnotation(config)

						_, err = kubeCl.CoreV1().
							Secrets(namespace).
							Update(ctx, config, metav1.UpdateOptions{})
					}

					return err
				})
		})
}

func addUnsafeAnnotation(doc *v1.Secret) {
	if doc.Annotations == nil {
		doc.Annotations = make(map[string]string)
	}
	doc.Annotations[allowUnsafeAnnotation] = "true"
}

func removeUnsafeAnnotation(doc *v1.Secret) {
	delete(doc.Annotations, allowUnsafeAnnotation)
}
