/*
Copyright 2024 Flant JSC

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

package provider

import "context"

// Identity is what a provider learned about the user whose credentials it validated.
type Identity struct {
	Groups []string
	// Subject and Generation are the sub and d8_gen claims of the ID token Dex issued for the user:
	// a revocation names the user by the subject and revokes everything below its generation. Both
	// are zero for the providers that do not go through Dex.
	Subject    string
	Generation uint64
}

// Provider validates user credentials against an upstream IdP and returns
// the user's identity on success. ctx must be propagated to outbound calls.
type Provider interface {
	ValidateCredentials(ctx context.Context, login, password string) (Identity, error)
}
