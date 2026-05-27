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

package client

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/task"
)

// Client is a client that executes commands on hosts using the OpenSSH client.
// It spawns tasks and stores their results by providerID.
type Client struct {
	taskManager         *task.Manager
	taskManagerCtx      context.Context
	taskManagerCancel   context.CancelFunc
	tcpCheckRateLimiter workqueue.TypedRateLimiter[string]

	client   k8sClient.Client
	recorder *event.Recorder
}

// NewClient creates a new Client.
func NewClient(recorder *event.Recorder, client k8sClient.Client) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		taskManagerCtx:      ctx,
		taskManagerCancel:   cancel,
		taskManager:         task.NewTaskManager(),
		tcpCheckRateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[string](250*time.Millisecond, time.Minute),
		client:              client,
		recorder:            recorder,
	}
}
