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
	"sync"

	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/providerid"
)

// Client is a client that executes commands on hosts using the OpenSSH client.
// It spawns tasks and stores their results by providerID.
type Client struct {
	tasksMutex sync.Mutex
	tasks      map[providerid.ProviderID]*bool

	recorder *event.Recorder
}

// NewClient creates a new Client.
func NewClient(recorder *event.Recorder) *Client {
	return &Client{
		tasks:    make(map[providerid.ProviderID]*bool),
		recorder: recorder,
	}
}

// spawn spawns a new task if it doesn't exist yet.
func (c *Client) spawn(providerID providerid.ProviderID, task func() bool) bool {
	c.tasksMutex.Lock()
	defer c.tasksMutex.Unlock()

	// Avoid spawning multiple tasks for the same providerID.
	done, ok := c.tasks[providerID]
	if ok {
		if done == nil {
			return false
		}

		delete(c.tasks, providerID)

		return *done
	}

	c.tasks[providerID] = nil

	go func() {
		var done bool

		defer func() {
			c.tasksMutex.Lock()
			defer c.tasksMutex.Unlock()

			c.tasks[providerID] = &done
		}()

		done = task()
	}()

	return false
}
