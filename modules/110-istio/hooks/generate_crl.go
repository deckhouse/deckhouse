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
	"github.com/tidwall/gjson"
)

const (
	crlConfigPath   = "istio.crl"
	internalCRLPath = "istio.internal.crl"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// After generate_ca (10) and alliance_metadata_merge (10): peer CRLs come
	// from federation/multicluster values populated by metadata exchange.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
}, publishCRL)

// publishCRL sets istio.internal.crl for rendering ca-crl.pem.
//
// Local part: only non-empty settings.crl.pem (manual delivery). No auto-dummy —
// absent settings.crl means no ca-crl.pem, so federation stays healthy until
// an operator explicitly enables CRL.
//
// Then append peer CRLs from istio.internal.federations[].crl and
// istio.internal.multiclusters[].crl (metadata exchange PoC) so remote
// issuers are covered for mTLS verify when CRL enforcement is on.
func publishCRL(_ context.Context, input *go_hook.HookInput) error {
	local := resolveLocalCRL(input)
	if local == "" {
		input.Values.Remove(internalCRLPath)
		return nil
	}

	input.Values.Set(internalCRLPath, concatCRLPEMs(local, collectPeerCRLs(input)))
	return nil
}

func resolveLocalCRL(input *go_hook.HookInput) string {
	if !input.Values.Exists(crlConfigPath) {
		return ""
	}
	v := input.Values.Get(crlConfigPath)
	if !v.IsObject() {
		return ""
	}
	return strings.TrimSpace(v.Get("pem").String())
}

func collectPeerCRLs(input *go_hook.HookInput) string {
	var b strings.Builder
	appendPeerCRLArray(&b, input.Values.Get("istio.internal.federations").Array())
	appendPeerCRLArray(&b, input.Values.Get("istio.internal.multiclusters").Array())
	return b.String()
}

func appendPeerCRLArray(b *strings.Builder, peers []gjson.Result) {
	for _, peer := range peers {
		crl := peer.Get("crl").String()
		if strings.TrimSpace(crl) == "" {
			continue
		}
		b.WriteString(strings.TrimSpace(crl))
		b.WriteByte('\n')
	}
}

func concatCRLPEMs(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return b.String()
}
