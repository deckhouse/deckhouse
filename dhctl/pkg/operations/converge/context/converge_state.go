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
	"context"
	"encoding/json"
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

const (
	stateSecretName = "d8-dhctl-converge-state"
)

// errConvergeStateTransient marks a transport/API-level failure while reading or deleting the
// converge-state secret, as opposed to a permanent authorization failure or malformed state
// data that will not resolve by retrying.
var errConvergeStateTransient = fmt.Errorf("converge state: transient error, may succeed on retry")

type State struct {
	Phase phases.OperationPhase `json:"phase"`

	// ConvergeUserNodes names the masters this converge created or recreated. They boot
	// with the converge user in their cloud-init payload, the masters already in the
	// cluster do not. Names only: this state is kept in a Secret in the cluster.
	ConvergeUserNodes []string `json:"convergeUserNodes,omitempty"`

	// ConvergeUserExpiry is when the accounts on those masters stop accepting logins. A
	// name carries no age of its own, and a cleanup skipped in commander or sshless mode
	// never prunes the list. A zero value is an expired one: it predates this field.
	ConvergeUserExpiry time.Time `json:"convergeUserExpiry,omitempty"`
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
	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return nil, fmt.Errorf("Could not get kube client: %w", err)
	}

	loopParams := retry.NewEmptyParams(
		retry.WithName("Get converge state from Kubernetes cluster"),
		retry.WithAttempts(25),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errConvergeStateTransient),
	)

	// Silent: every switch, every credentials pick and the deletion at the end of converge
	// read this state, and a process block each would bury the converge's own output.
	err = retry.NewSilentLoopWithParams(loopParams).RunContext(ctx.Ctx(), func() error {
		c, cancel := ctx.WithTimeout(10 * time.Second)
		defer cancel()

		convergeStateSecret, err := kubeClient.CoreV1().Secrets("d8-system").Get(c, stateSecretName, metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			if kubeerrors.IsPermanentAuthError(c, err) {
				return fmt.Errorf("failed to get secret: %w", err)
			}

			return fmt.Errorf("%w: failed to get secret: %w", errConvergeStateTransient, err)
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
	kubeClient, err := ctx.KubeClientCtx(ctx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}
	loopParams := retry.NewEmptyParams(
		retry.WithName("Cleanup converge state from Kubernetes cluster"),
		retry.WithAttempts(25),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(errConvergeStateTransient),
	)

	return retry.NewLoopWithParams(loopParams).RunContext(ctx.Ctx(), func() error {
		c, cancel := ctx.WithTimeout(10 * time.Second)
		defer cancel()

		err := kubeClient.CoreV1().Secrets("d8-system").Delete(c, stateSecretName, metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				return nil
			}

			if kubeerrors.IsPermanentAuthError(c, err) {
				return fmt.Errorf("failed to delete state secret: %w", err)
			}

			return fmt.Errorf("%w: failed to delete state secret: %w", errConvergeStateTransient, err)
		}

		return nil
	})
}

func (s *inSecretStateStore) SetState(convergeCtx *Context, state *State) error {
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	kubeClient, err := convergeCtx.KubeClientCtx(convergeCtx.Ctx())
	if err != nil {
		return fmt.Errorf("Could not get kube client: %w", err)
	}

	task := actions.ManifestTask{
		Name: fmt.Sprintf(`Secret "%s"`, stateSecretName),
		Manifest: func() any {
			return manifests.SecretConvergeState(stateBytes)
		},
		CreateFunc: func(ctx context.Context, manifest any) error {
			c, cancel := convergeCtx.WithTimeout(10 * time.Second)
			defer cancel()

			_, err := kubeClient.CoreV1().Secrets("d8-system").Create(c, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}

			return nil
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			c, cancel := convergeCtx.WithTimeout(10 * time.Second)
			defer cancel()

			_, err := kubeClient.CoreV1().Secrets("d8-system").Update(c, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update secret: %w", err)
			}

			return nil
		},
	}

	loopParams := retry.NewEmptyParams(
		retry.WithName("Save dhctl converge state"),
		retry.WithAttempts(450),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(actions.ErrManifestTaskTransient),
	)

	return retry.NewLoopWithParams(loopParams).
		RunContext(
			convergeCtx.Ctx(),
			func() error {
				return task.CreateOrUpdate(convergeCtx.Ctx())
			},
		)
}
