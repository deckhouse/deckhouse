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

package hooks

import (
	"context"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

// The chart copies the custom certificate only once internal.customCertificateData holds it, because
// addon-operator also renders the chart before the hooks run (see templates/custom-certificate.yaml).
// Running after copy_custom_certificate (order 10), this hook fails the module run in the
// CustomCertificate mode when that hook left the certificate out, the way the helm_lib helper used to
// fail the render.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
}, requireCustomCertificate)

func requireCustomCertificate(_ context.Context, input *go_hook.HookInput) error {
	if module.GetHTTPSMode("userAuthn", input) != "CustomCertificate" {
		return nil
	}

	certificate := input.Values.Get("userAuthn.internal.customCertificateData")
	if certificate.Get(`tls\.crt`).String() == "" || certificate.Get(`tls\.key`).String() == "" {
		return errors.New("internal.customCertificateData is required to copy custom certificate for secret prefix 'ingress-tls': " +
			"with https.mode CustomCertificate, https.customCertificate.secretName must name a kubernetes.io/tls Secret in d8-system")
	}

	return nil
}
