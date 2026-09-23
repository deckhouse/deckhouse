// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"time"

	klient "github.com/flant/kube-client/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const configMapLockName = "deckhouse-bootstrap-lock"

func lockUntilClusterBootstraped(ctx context.Context, logger *log.Logger) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.2,
		Jitter:   1,
		Steps:    10,
		Cap:      5 * time.Minute,
	}

	retriable := func(err error) bool {
		logger.Error("error occurred during the bootstrap lock, retry", log.Err(err))
		// retry on any error
		return true
	}

	client := klient.New()

	return retry.OnError(backoff, retriable, func() error {
		if _, err := client.CoreV1().ConfigMaps(app.NamespaceDeckhouse).Get(ctx, configMapLockName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get configmap '%s': %w", configMapLockName, err)
		}

		logger.Info("bootstrap lock config map exists, wait for bootstrap")

		opts := v1.ListOptions{
			FieldSelector: "metadata.name=" + configMapLockName,
			Watch:         true,
		}
		wch, err := client.CoreV1().ConfigMaps(app.NamespaceDeckhouse).Watch(ctx, opts)
		if err != nil {
			return fmt.Errorf("watch configmaps: %w", err)
		}

		for event := range wch.ResultChan() {
			if event.Type == watch.Deleted {
				break
			}
		}
		wch.Stop()

		logger.Info("bootstrap lock has been released")

		return nil
	})
}
