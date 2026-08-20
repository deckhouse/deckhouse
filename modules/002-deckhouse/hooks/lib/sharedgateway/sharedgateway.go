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

// Package sharedgateway names the Gateway object the Deckhouse modules attach their own Gateway API
// routes to.
//
// helm_lib_module_gateway (helm_lib/charts/deckhouse_lib_helm/templates/_module_gateway.tpl) is
// where every module reads that name from, and this package answers the same question in Go for the
// hooks that cannot call a Helm template. templates/reserved-public-hosts.yaml calls the helper
// itself and publishes the answer in the ConfigMap, and
// template_tests/reserved_public_hosts_test.go compares the two so that they cannot drift: an
// exemption that named a different object from the one the platform attaches to would either leave
// the shared Gateway unwritable or exempt something else.
package sharedgateway

// Ref is a Gateway object by identity. Hostnames have nothing to do with it: the reservation lets
// this one object through because of what it is, not because of what it claims.
type Ref struct {
	Namespace string
	Name      string
}

// Resolve picks the Gateway helm_lib_module_gateway would pick. Each argument is one of the three
// places that helper looks, in its order of precedence, and nil says the key is absent there.
//
// A reference missing either half yields no Gateway, and the places after it are not consulted --
// which is the helper's behaviour and not an accident of it: a half-written reference is one the
// platform attaches nothing to either, so nothing about it should be exempt from anything. The
// global schema requires both halves (global-hooks/openapi/config-values.yaml,
// global.modules.gatewayAPIGateway), so this is what a chart rendered without that schema does,
// while global-hooks/discovery/default_gateway.go writes both halves empty whenever it finds no
// gateway to discover.
func Resolve(module, global, discovered *Ref) Ref {
	for _, configured := range []*Ref{module, global, discovered} {
		if configured == nil {
			continue
		}
		if configured.Namespace == "" || configured.Name == "" {
			return Ref{}
		}
		return *configured
	}
	return Ref{}
}

// String is the spelling the sharedGateway key of the d8-reserved-public-hosts ConfigMap carries,
// which is namespace/name -- the same spelling the default-gateway ConfigMap in d8-alb uses. It is
// empty for anything but a complete reference, so that "no shared Gateway" cannot read as "every
// object".
func (r Ref) String() string {
	if r.Namespace == "" || r.Name == "" {
		return ""
	}
	return r.Namespace + "/" + r.Name
}

// Is reports whether the object is that Gateway. An incomplete reference is no object.
func (r Ref) Is(namespace, name string) bool {
	return r.String() != "" && r.Namespace == namespace && r.Name == name
}
