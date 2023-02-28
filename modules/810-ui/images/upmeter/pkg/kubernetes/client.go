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

package kubernetes

import (
	kube "github.com/flant/kube-client/client"
)

func InitKubeClient(config *Config) (kube.Client, error) {
	client := kube.New()

	client.WithContextName(config.Context)
	client.WithConfigPath(config.Config)
	client.WithRateLimiterSettings(config.ClientQps, config.ClientBurst)
	// TODO(nabokihms): add kubernetes client metrics

	// FIXME: Kubernetes client is configured successfully with 'out-of-cluster' config
	//      operator.component=KubernetesAPIClient
	return client, client.Init()
}
