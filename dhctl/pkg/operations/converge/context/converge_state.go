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

package context

import (
	"encoding/json"
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	stateSecretName = "d8-dhctl-converge-state"
)

type State struct {
	Phase               phases.OperationPhase `json:"phase"`
	NodeUserCredentials *NodeUserCredentials  `json:"nodeUserCredentials"`
}

type stateStore interface {
	GetState(ctx *Context) (*State, error)
	SetState(ctx *Context, st *State) error
	Delete(ctx *Context) error
}

type inSecretStateStore struct{}

func newInSecretStateStore() *inSecretStateStore {
	return &inSecretStateStore{}
}

func (s *inSecretStateStore) GetState(ctx *Context) (*State, error) {
	var state State

	err := retry.NewLoop("Get converge state from Kubernetes cluster", 5, 5*time.Second).RunContext(ctx.Ctx(), func() error {
		c, cancel := ctx.WithTimeout(10 * time.Second)
		defer cancel()

		convergeStateSecret, err := ctx.KubeClient().CoreV1().Secrets("d8-system").Get(c, stateSecretName, metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to get secret: %w", err)
		}

		err = json.Unmarshal(convergeStateSecret.Data["state.json"], &state)
		if err != nil {
			return fmt.Errorf("failed to unmarshal state: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get `%s` secret: %w", stateSecretName, err)
	}

	return &state, nil
}

func (s *inSecretStateStore) Delete(ctx *Context) error {
	return retry.NewLoop("Cleanup converge state from Kubernetes cluster", 5, 5*time.Second).RunContext(ctx.Ctx(), func() error {
		c, cancel := ctx.WithTimeout(10 * time.Second)
		defer cancel()

		err := ctx.KubeClient().CoreV1().Secrets("d8-system").Delete(c, stateSecretName, metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to delete state secret: %w", err)
		}

		return nil
	})
}

func (s *inSecretStateStore) SetState(ctx *Context, state *State) error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "%s"`, stateSecretName),
		Manifest: func() interface{} {
			return manifests.SecretConvergeState(stateBytes)
		},
		CreateFunc: func(manifest interface{}) error {
			c, cancel := ctx.WithTimeout(10 * time.Second)
			defer cancel()

			_, err := ctx.KubeClient().CoreV1().Secrets("d8-system").Create(c, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}

			return nil
		},
		UpdateFunc: func(manifest interface{}) error {
			c, cancel := ctx.WithTimeout(10 * time.Second)
			defer cancel()

			_, err := ctx.KubeClient().CoreV1().Secrets("d8-system").Update(c, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update secret: %w", err)
			}

			return nil
		},
	}

	return retry.NewLoop("Save dhctl converge state", 45, 10*time.Second).RunContext(ctx.Ctx(), task.CreateOrUpdate)
}
