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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

const (
	crlConfigPath   = "istio.crl"
	internalCRLPath = "istio.internal.crl"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
}, publishCRL)

// publishCRL copies ModuleConfig settings.crl (PEM) to istio.internal.crl
// for rendering Secret d8-istio/cacerts ca-crl.pem. Empty/unset means no
// ca-crl.pem (CRL enforcement off). No federation merge, no dummy CRL.
func publishCRL(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Exists(crlConfigPath) {
		input.Values.Remove(internalCRLPath)
		return nil
	}

	pem := strings.TrimSpace(input.Values.Get(crlConfigPath).String())
	if pem == "" {
		input.Values.Remove(internalCRLPath)
		return nil
	}

	input.Values.Set(internalCRLPath, pem)
	return nil
}
