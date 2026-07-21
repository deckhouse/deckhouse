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

package hooks

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const cn = "capi-controller-manager-webhook"

func genWebhookCa(logEntry *log.Logger) (*certificate.Authority, error) {
	ca, err := certificate.GenerateCA(logEntry, cn, certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	if err != nil {
		return nil, fmt.Errorf("cannot generate CA: %v", err)
	}

	return &ca, nil
}

func genWebhookTLS(input *go_hook.HookInput, ca *certificate.Authority, cn string, sanPrefix string) (*certificate.Certificate, error) {
	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		cn,
		*ca,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry((24*time.Hour)*365*10),
		certificate.WithSigningDefaultUsage([]string{"signing",
			"key encipherment",
			"requestheader-client",
		}),
		certificate.WithSANs(
			sanPrefix+".d8-cloud-instance-manager",
			sanPrefix+".d8-cloud-instance-manager.svc",
			sanPrefix+".d8-cloud-instance-manager."+clusterDomain,
			sanPrefix+".d8-cloud-instance-manager.svc."+clusterDomain,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot generate TLS: %v", err)
	}

	return &tls, err
}
