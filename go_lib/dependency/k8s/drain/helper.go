/*
Copyright 2021 Flant JSC

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

package drain

import (
	"io"
	"time"

	"k8s.io/client-go/kubernetes"
)

type HelperConfig struct {
	Client              kubernetes.Interface
	Force               *bool
	IgnoreAllDaemonSets *bool
	DeleteEmptyDirData  *bool
	GracePeriodSeconds  *int
	Timeout             *time.Duration
}

func NewDrainer(config HelperConfig) *Helper {
	drainer := &Helper{
		Client:              config.Client,
		Force:               true,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true, // same as DeleteLocalData
		GracePeriodSeconds:  -1,
		// If a pod is not evicted in 5 minutes, delete pod
		Timeout: 5 * time.Minute,
		Out:     io.Discard,
		ErrOut:  io.Discard,
	}

	if config.Force != nil {
		drainer.Force = *config.Force
	}
	if config.IgnoreAllDaemonSets != nil {
		drainer.IgnoreAllDaemonSets = *config.IgnoreAllDaemonSets
	}
	if config.DeleteEmptyDirData != nil {
		drainer.DeleteEmptyDirData = *config.DeleteEmptyDirData
	}
	if config.GracePeriodSeconds != nil {
		drainer.GracePeriodSeconds = *config.GracePeriodSeconds
	}
	if config.Timeout != nil {
		drainer.Timeout = *config.Timeout
	}

	return drainer
}
