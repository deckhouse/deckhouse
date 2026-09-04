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

package v1alpha1

import (
	"fmt"
	"sort"
)

// The resources this module derives name their credentials instead of carrying them, and
// two components have to turn a name back into a credential: the node agent and the storage
// syncer. What follows is the part of that they share.
//
// Kept here, as pure functions over the types, for two reasons. The rules are about the
// types themselves — which fields hold a reference, what replaces it, when a missing key is
// an error — so the types are where they belong. And a second copy of them in another
// module would be a second chance to get a credential path subtly wrong; these two
// components are the ones that authenticate every image pull in the cluster.
//
// Fetching is left to the callers, because they have very different clients: one is a static
// pod that has to keep working with no API server at all.

// ReferencedAuths returns the credentials this layout still needs resolved, as pointers into
// the spec so that filling them in fills in the layout.
func (s *RegistryNodeSpec) ReferencedAuths() []*Auth {
	if s == nil {
		return nil
	}

	var out []*Auth
	for i := range s.Backends {
		out = appendUnresolved(out, s.Backends[i].Endpoint.Auth)
		for j := range s.Backends[i].Mirrors {
			out = appendUnresolved(out, s.Backends[i].Mirrors[j].Auth)
		}
	}
	for i := range s.AdditionalRoutes {
		out = appendUnresolved(out, s.AdditionalRoutes[i].Endpoint.Auth)
		for j := range s.AdditionalRoutes[i].Mirrors {
			out = appendUnresolved(out, s.AdditionalRoutes[i].Mirrors[j].Auth)
		}
	}
	return sortedAuths(out)
}

// ReferencedAuths returns the credentials this storage spec still needs resolved.
func (s *RegistryStorageSpec) ReferencedAuths() []*Auth {
	if s == nil || s.Upstream == nil {
		return nil
	}

	out := appendUnresolved(nil, s.Upstream.Endpoint.Auth)
	for i := range s.Upstream.Mirrors {
		out = appendUnresolved(out, s.Upstream.Mirrors[i].Auth)
	}
	return sortedAuths(out)
}

// SecretNames returns the Secrets a set of references points at, each once.
//
// One credential is typically shared by many endpoints — every replica of the cache takes
// the same one — and fetching per endpoint would turn a single layout into an API call per
// address on every pass.
func SecretNames(auths []*Auth) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, auth := range auths {
		name := auth.SecretRef.Name
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ResolveAuths replaces every reference with the credential it names, given the contents of
// the Secrets it points at keyed by Secret name.
//
// A missing key is an error rather than an endpoint left without credentials. Both outcomes
// produce a layout that looks well-formed; only one of them says why the pull that follows
// fails with a 401.
func ResolveAuths(auths []*Auth, secrets map[string]map[string][]byte) error {
	for _, auth := range auths {
		data, ok := secrets[auth.SecretRef.Name]
		if !ok {
			return fmt.Errorf("no contents supplied for secret %q", auth.SecretRef.Name)
		}
		value, ok := data[auth.SecretRef.Key]
		if !ok {
			return fmt.Errorf("secret %q has no key %q", auth.SecretRef.Name, auth.SecretRef.Key)
		}
		if len(value) == 0 {
			return fmt.Errorf("secret %q holds nothing under key %q",
				auth.SecretRef.Name, auth.SecretRef.Key)
		}

		// The stored form is already base64("username:password") — the form a containerd
		// hosts.toml wants. Decoding it here only to re-encode downstream would add a
		// place where a password containing a colon splits differently.
		auth.Auth = string(value)
		auth.Username = ""
		auth.Password = ""
		// Cleared so that nothing downstream can mistake a resolved credential for one
		// that still needs a lookup, and so a resolved layout written to disk carries no
		// reference it could never dereference again.
		auth.SecretRef = nil
	}
	return nil
}

func appendUnresolved(into []*Auth, auth *Auth) []*Auth {
	if auth.NeedsResolution() {
		return append(into, auth)
	}
	return into
}

// sortedAuths orders references so that a failure names the same key every time, rather than
// whichever one map iteration reached first.
func sortedAuths(auths []*Auth) []*Auth {
	sort.SliceStable(auths, func(i, j int) bool {
		if auths[i].SecretRef.Name != auths[j].SecretRef.Name {
			return auths[i].SecretRef.Name < auths[j].SecretRef.Name
		}
		return auths[i].SecretRef.Key < auths[j].SecretRef.Key
	})
	return auths
}
