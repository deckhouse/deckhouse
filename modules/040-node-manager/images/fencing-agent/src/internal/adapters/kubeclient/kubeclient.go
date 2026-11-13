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

package kubeclient

import (
	"net"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const clientQPS = -1

// dialer limits the TCP connect to the API server to 5 s. The client-go default is 30 s.
// When the node starts, the pod network can be not ready, and a connect waits until the timeout.
// A short timeout lets the failed requests try again soon after the network is ready.
var dialer = &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}

func NewRestConfig() (*rest.Config, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	cfg.QPS = clientQPS
	cfg.Dial = dialer.DialContext

	return cfg, nil
}

func New(cfg *rest.Config) (kubernetes.Interface, error) {
	cfg = rest.CopyConfig(cfg)
	cfg.ContentType = runtime.ContentTypeProtobuf

	return kubernetes.NewForConfig(cfg)
}
