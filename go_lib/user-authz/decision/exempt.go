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

package decision

import "strings"

// exemptUsers are the identities the API server never asks the authorization webhook about.
//
// They mirror the matchConditions of the Webhook authorizer in
// modules/040-control-plane-manager/templates/_authorization_config.tpl, which is the source of
// truth; TestExemptionsMirrorTheAuthorizationConfig reads that file and fails when the two stop
// agreeing, so a change there cannot land quietly here.
var exemptUsers = map[string]struct{}{
	"system:aggregator":              {},
	"system:kube-aggregator":         {},
	"system:kube-controller-manager": {},
	"system:kube-scheduler":          {},
	"kubernetes-admin":               {},
	"kube-apiserver-kubelet-client":  {},
	"capi-controller-manager":        {},
	"system:volume-scheduler":        {},
}

// exemptPrefixes are the identity prefixes the same matchConditions exclude.
var exemptPrefixes = []string{
	"system:node:",
	"system:serviceaccount:kube-system:",
	"system:serviceaccount:d8-",
}

// ExemptFromWebhook reports whether the API server decides this subject's requests without ever
// asking the authorization webhook.
//
// The platform excludes the control plane's own identities from the webhook, because a fail-closed
// authorizer that the control plane depends on is a way for a cluster to become unrecoverable: if
// kube-controller-manager or a Deckhouse module cannot be authorized, nothing is left that could
// repair the webhook. So for these subjects a ClusterAuthorizationRule's namespace limits are not
// enforced at all - the access level applies cluster-wide, system namespaces included.
//
// This is here for the consumers that are NOT the webhook. The webhook itself never needs it: it is
// simply not asked. permission-browser is asked about everybody, and reporting a limit the API
// server does not enforce is the same class of mistake as enforcing one it does not - which is what
// this package exists to prevent.
//
// A rule does not have to name these subjects deliberately for this to matter: a rule whose
// subjects include a group like system:authenticated covers every service account in the cluster.
func ExemptFromWebhook(username string) bool {
	if _, ok := exemptUsers[username]; ok {
		return true
	}
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(username, prefix) {
			return true
		}
	}
	return false
}
