/*
Copyright 2021 Flant CJSC

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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/etcd"
)

const (
	moduleQueue = "/modules/control-plane-manager"
)

func getETCDClient(input *go_hook.HookInput, dc dependency.Container, endpoints []string) (etcd.Client, error) {
	ca := input.Values.Get("controlPlaneManager.internal.etcdCerts.ca").String()
	crt := input.Values.Get("controlPlaneManager.internal.etcdCerts.crt").String()
	key := input.Values.Get("controlPlaneManager.internal.etcdCerts.key").String()

	if ca == "" || crt == "" || key == "" {
		return nil, fmt.Errorf("etcd credentials not found")
	}

	caCert, clientCert, err := certificate.ParseCertificatesFromBase64(ca, crt, key)
	if err != nil {
		return nil, err
	}

	return dc.GetEtcdClient(endpoints, etcd.WithClientCert(clientCert, caCert), etcd.WithInsecureSkipVerify())
}
