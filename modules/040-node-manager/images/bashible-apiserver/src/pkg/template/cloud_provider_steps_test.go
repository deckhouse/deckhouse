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

package template

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestCloudProviderStepsFor(t *testing.T) {
	tests := []struct {
		name            string
		secrets         map[string]cloudProviderStepsSecret
		provider        string
		expectedScripts map[string][]byte
		expectedFound   bool
		expectedError   bool
	}{
		{
			name:     "provider Secret is absent",
			secrets:  map[string]cloudProviderStepsSecret{},
			provider: "aws",

			expectedScripts: map[string][]byte{},
			expectedFound:   false,
		},
		{
			name: "empty provider Secret exists",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-steps": {
					provider: "aws",
					data:     map[string][]byte{},
				},
			},
			provider: "aws",

			expectedScripts: map[string][]byte{},
			expectedFound:   true,
		},
		{
			name: "returns scripts only for requested provider",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-steps": {
					provider: "aws",
					data: map[string][]byte{
						"001_aws.sh.tpl": []byte("aws"),
					},
				},
				"azure-steps": {
					provider: "azure",
					data: map[string][]byte{
						"001_azure.sh.tpl": []byte("azure"),
					},
				},
			},
			provider: "aws",

			expectedScripts: map[string][]byte{
				"001_aws.sh.tpl": []byte("aws"),
			},
			expectedFound: true,
		},
		{
			name: "combines multiple Secrets of one provider",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-first": {
					provider: "aws",
					data: map[string][]byte{
						"001_first.sh.tpl": []byte("first"),
					},
				},
				"aws-second": {
					provider: "aws",
					data: map[string][]byte{
						"002_second.sh.tpl": []byte("second"),
					},
				},
			},
			provider: "aws",

			expectedScripts: map[string][]byte{
				"001_first.sh.tpl":  []byte("first"),
				"002_second.sh.tpl": []byte("second"),
			},
			expectedFound: true,
		},
		{
			name: "rejects duplicate script names",
			secrets: map[string]cloudProviderStepsSecret{
				"aws-first": {
					provider: "aws",
					data: map[string][]byte{
						"001_same.sh.tpl": []byte("first"),
					},
				},
				"aws-second": {
					provider: "aws",
					data: map[string][]byte{
						"001_same.sh.tpl": []byte("second"),
					},
				},
			},
			provider:      "aws",
			expectedFound: true,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &StepsStorage{
				cloudProviderStepSecrets: tt.secrets,
			}

			scripts, found, err := storage.cloudProviderStepsFor(tt.provider)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedScripts, scripts)
			}

			require.Equal(t, tt.expectedFound, found)
		})
	}
}

func TestUpsertCloudProviderStepsSecret(t *testing.T) {
	storage := &StepsStorage{
		cloudProviderStepSecrets:  make(map[string]cloudProviderStepsSecret),
		cloudProviderStepsChanged: make(chan struct{}, 1),
	}

	originalContent := []byte("original")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aws-steps",
			Labels: map[string]string{
				cloudProviderBashibleLabel: "steps",
				cloudProviderNameLabel:     "aws",
			},
		},
		Data: map[string][]byte{
			"001_step.sh.tpl": originalContent,
			"ignored.txt":     []byte("must not be included"),
		},
	}

	storage.upsertCloudProviderStepsSecret(secret)

	stored, exists := storage.cloudProviderStepSecrets["aws-steps"]
	require.True(t, exists)
	require.Equal(t, "aws", stored.provider)
	require.Equal(t, []byte("original"), stored.data["001_step.sh.tpl"])
	require.NotContains(t, stored.data, "ignored.txt")

	select {
	case <-storage.cloudProviderStepsChanged:
	default:
		t.Fatal("expected cloud-provider steps change notification")
	}

	// The storage must own a copy and not reference informer-managed data.
	originalContent[0] = 'X'
	require.Equal(t, []byte("original"), stored.data["001_step.sh.tpl"])

	// Upsert replaces the complete contents of the Secret rather than merging it
	// with the previous version.
	secret.Data = map[string][]byte{
		"002_new_step.sh.tpl": []byte("new"),
	}
	storage.upsertCloudProviderStepsSecret(secret)

	stored = storage.cloudProviderStepSecrets["aws-steps"]
	require.NotContains(t, stored.data, "001_step.sh.tpl")
	require.Equal(t, []byte("new"), stored.data["002_new_step.sh.tpl"])
}

func TestUpsertCloudProviderStepsSecretWithoutProviderLabel(t *testing.T) {
	storage := &StepsStorage{
		cloudProviderStepSecrets:  make(map[string]cloudProviderStepsSecret),
		cloudProviderStepsChanged: make(chan struct{}, 1),
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-steps",
			Labels: map[string]string{
				cloudProviderBashibleLabel: "steps",
			},
		},
		Data: map[string][]byte{
			"001_step.sh.tpl": []byte("step"),
		},
	}

	storage.upsertCloudProviderStepsSecret(secret)

	require.Empty(t, storage.cloudProviderStepSecrets)

	select {
	case <-storage.cloudProviderStepsChanged:
		t.Fatal("unexpected change notification")
	default:
	}
}

func TestDeleteCloudProviderStepsSecret(t *testing.T) {
	storage := &StepsStorage{
		cloudProviderStepSecrets: map[string]cloudProviderStepsSecret{
			"aws-steps": {
				provider: "aws",
				data: map[string][]byte{
					"001_step.sh.tpl": []byte("step"),
				},
			},
		},
		cloudProviderStepsChanged: make(chan struct{}, 1),
	}

	storage.deleteCloudProviderStepsSecret("aws-steps")

	require.NotContains(t, storage.cloudProviderStepSecrets, "aws-steps")

	select {
	case <-storage.cloudProviderStepsChanged:
	default:
		t.Fatal("expected cloud-provider steps change notification")
	}

	// Repeated deletion changes nothing and must not trigger an update.
	storage.deleteCloudProviderStepsSecret("aws-steps")

	select {
	case <-storage.cloudProviderStepsChanged:
		t.Fatal("unexpected change notification")
	default:
	}
}

func TestDeletedSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "aws-steps",
		},
	}

	t.Run("direct Secret", func(t *testing.T) {
		result, ok := deletedSecret(secret)

		require.True(t, ok)
		require.Same(t, secret, result)
	})

	t.Run("tombstone", func(t *testing.T) {
		result, ok := deletedSecret(cache.DeletedFinalStateUnknown{
			Key: "kube-system/aws-steps",
			Obj: secret,
		})

		require.True(t, ok)
		require.Same(t, secret, result)
	})

	t.Run("unexpected object", func(t *testing.T) {
		result, ok := deletedSecret("not a Secret")

		require.False(t, ok)
		require.Nil(t, result)
	})
}

func TestSubscribeOnCloudProviderSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	matchingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "aws-steps",
			Namespace:       cloudProviderStepsSecretNamespace,
			ResourceVersion: "1",
			Labels: map[string]string{
				cloudProviderBashibleLabel: "steps",
				cloudProviderNameLabel:     "aws",
			},
		},
		Data: map[string][]byte{
			"001_aws.sh.tpl": []byte("initial"),
		},
	}

	ignoredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azure-bootstrap",
			Namespace: cloudProviderStepsSecretNamespace,
			Labels: map[string]string{
				cloudProviderBashibleLabel: "bootstrap",
				cloudProviderNameLabel:     "azure",
			},
		},
		Data: map[string][]byte{
			"001_azure.sh.tpl": []byte("must be ignored"),
		},
	}

	kubeClient := fake.NewSimpleClientset(
		matchingSecret,
		ignoredSecret,
	)

	factory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		0,
		informers.WithNamespace(cloudProviderStepsSecretNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = cloudProviderStepsLabelSelector
		}),
	)

	storage := &StepsStorage{
		cloudProviderStepSecrets:  make(map[string]cloudProviderStepsSecret),
		cloudProviderStepsChanged: make(chan struct{}, 1),
	}

	storage.subscribeOnCloudProviderSteps(ctx, factory)

	t.Run("loads matching Secret during initial list", func(t *testing.T) {
		scripts, found, err := storage.cloudProviderStepsFor("aws")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, []byte("initial"), scripts["001_aws.sh.tpl"])
	})

	t.Run("ignores Secret outside selector", func(t *testing.T) {
		scripts, found, err := storage.cloudProviderStepsFor("azure")

		require.NoError(t, err)
		require.False(t, found)
		require.Empty(t, scripts)
	})

	t.Run("receives Secret update through watch", func(t *testing.T) {
		updatedSecret := matchingSecret.DeepCopy()
		updatedSecret.ResourceVersion = "2"
		updatedSecret.Data = map[string][]byte{
			"002_aws.sh.tpl": []byte("updated"),
		}

		_, err := kubeClient.CoreV1().
			Secrets(cloudProviderStepsSecretNamespace).
			Update(ctx, updatedSecret, metav1.UpdateOptions{})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			scripts, found, err := storage.cloudProviderStepsFor("aws")
			if err != nil || !found {
				return false
			}

			_, oldExists := scripts["001_aws.sh.tpl"]
			return !oldExists &&
				string(scripts["002_aws.sh.tpl"]) == "updated"
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("receives Secret deletion through watch", func(t *testing.T) {
		err := kubeClient.CoreV1().
			Secrets(cloudProviderStepsSecretNamespace).
			Delete(ctx, matchingSecret.Name, metav1.DeleteOptions{})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			_, found, err := storage.cloudProviderStepsFor("aws")
			return err == nil && !found
		}, time.Second, 10*time.Millisecond)
	})
}
