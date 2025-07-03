/*
Copyright 2023 Flant JSC

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
	"time"

	"caps-controller-manager/internal/event"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Client is a client that executes commands on hosts using the OpenSSH client.
// It spawns tasks and stores their results by providerID.
type Client struct {
	checkTaskManager     *taskManager
	bootstrapTaskManager *taskManager
	cleanupTaskManager   *taskManager
	adoptTaskManager     *taskManager
	tcpCheckTaskManager  *taskManager
	tcpCheckRateLimiter  workqueue.RateLimiter

	recorder *event.Recorder
}

// NewClient creates a new Client.
func NewClient(recorder *event.Recorder) *Client {
	return &Client{
		checkTaskManager:     newTaskManager(ctrl.Log.WithName("checkTaskManager")),
		bootstrapTaskManager: newTaskManager(ctrl.Log.WithName("bootstrapTaskManager")),
		cleanupTaskManager:   newTaskManager(ctrl.Log.WithName("cleanupTaskManager")),
		adoptTaskManager:     newTaskManager(ctrl.Log.WithName("adoptTaskManager")),
		tcpCheckTaskManager:  newTaskManager(ctrl.Log.WithName("tcpCheckTaskManager")),
		tcpCheckRateLimiter:  workqueue.NewItemExponentialFailureRateLimiter(250*time.Millisecond, time.Minute),
		recorder:             recorder,
	}
}
