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

package useraccount

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"user-authn-controller/api/v1alpha1"
)

// ProviderTypeLocal is the key of TwoFactorPolicy for the local users; the other keys are the types of
// DexProvider (LDAP, Crowd).
const ProviderTypeLocal = "Local"

// TwoFactorRequirement is what the settings ask the users of one type of provider for: the methods, and with
// two of them how they combine.
type TwoFactorRequirement struct {
	Methods []string `json:"methods"`
	Require string   `json:"require,omitempty"`
	// MaxCredentials is how many WebAuthn keys a user may register; it is the same for every type, and the
	// controller sets it from its own variable, not from the policy.
	MaxCredentials int `json:"-"`
}

// TwoFactorPolicy is the second factors of each type of provider twoFactor covers, by the type: Local, LDAP,
// Crowd. A type it does not name has no second factor. The template renders it from the settings, the same
// map the Dex config is rendered from.
type TwoFactorPolicy map[string]TwoFactorRequirement

// ParseTwoFactorPolicy reads TwoFactorPolicy from its JSON. An empty value is no policy: 2FA is off.
func ParseTwoFactorPolicy(value string) (TwoFactorPolicy, error) {
	if value == "" {
		return nil, nil
	}
	var policy TwoFactorPolicy
	if err := json.Unmarshal([]byte(value), &policy); err != nil {
		return nil, fmt.Errorf("decode the policy of the second factor: %w", err)
	}
	for providerType, req := range policy {
		if err := req.validate(); err != nil {
			return nil, fmt.Errorf("the second factor of %s: %w", providerType, err)
		}
	}
	return policy, nil
}

func (r TwoFactorRequirement) validate() error {
	if len(r.Methods) == 0 || len(r.Methods) > 2 {
		return fmt.Errorf("methods %v must list one or two methods", r.Methods)
	}
	for i, m := range r.Methods {
		if m != v1alpha1.TwoFactorMethodTOTP && m != v1alpha1.TwoFactorMethodWebAuthn {
			return fmt.Errorf("unknown method %q", m)
		}
		if slices.Contains(r.Methods[:i], m) {
			return fmt.Errorf("method %q listed twice", m)
		}
	}
	switch {
	case len(r.Methods) == 2 && r.Require != v1alpha1.TwoFactorRequireAll && r.Require != v1alpha1.TwoFactorRequireAny:
		return fmt.Errorf("require %q of two methods must be %s or %s", r.Require, v1alpha1.TwoFactorRequireAll, v1alpha1.TwoFactorRequireAny)
	case len(r.Methods) == 1 && r.Require != "":
		return fmt.Errorf("require %q is not allowed with one method", r.Require)
	}
	return nil
}

// For returns the requirement of the users of providerType, or nil when the second factor does not cover it.
func (p TwoFactorPolicy) For(providerType string) *TwoFactorRequirement {
	req, ok := p[providerType]
	if !ok {
		return nil
	}
	return &req
}

// ParseMaxCredentials reads the limit of WebAuthn keys of a user; an empty value is no limit to show.
func ParseMaxCredentials(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("the limit of keys %q must be a positive integer", value)
	}
	return n, nil
}

// storedFactors is what Dex keeps of the second factors of an account, as the informer cache keeps it: whether
// there is a TOTP secret and whether a code confirmed it, and the number of WebAuthn credentials.
type storedFactors struct {
	TOTP          bool
	TOTPConfirmed bool
	WebAuthnKeys  int
}

// has tells whether the account has method, and whether it is complete: a TOTP secret confirmed by a code, or a
// WebAuthn credential, which is complete at the registration.
func (f storedFactors) has(method string) (bool, bool) {
	switch method {
	case v1alpha1.TwoFactorMethodTOTP:
		return f.TOTP, f.TOTP && f.TOTPConfirmed
	case v1alpha1.TwoFactorMethodWebAuthn:
		return f.WebAuthnKeys > 0, f.WebAuthnKeys > 0
	default:
		return false, false
	}
}

// twoFactorStatus is status.twoFactor of an account with the stored factors f, whose type of provider asks for
// req; nil when the second factor does not cover the type (req is nil), whatever Dex keeps of an earlier one. A
// secret or a key of a method no longer asked for shows in its block but does not count.
func twoFactorStatus(f storedFactors, req *TwoFactorRequirement) *v1alpha1.UserAccountTwoFactor {
	if req == nil {
		return nil
	}
	status := &v1alpha1.UserAccountTwoFactor{}
	if f.TOTP {
		status.TOTP = &v1alpha1.UserAccountTOTP{Confirmed: f.TOTPConfirmed}
	}
	if f.WebAuthnKeys > 0 {
		status.WebAuthn = &v1alpha1.UserAccountWebAuthn{Credentials: f.WebAuthnKeys, MaxCredentials: req.MaxCredentials}
	}

	status.Methods = slices.Clone(req.Methods)
	status.Require = req.Require
	if req.Require == v1alpha1.TwoFactorRequireAny {
		// One method of the user's choice: the account needs any of them.
		for _, m := range req.Methods {
			enrolled, confirmed := f.has(m)
			status.Enrolled = status.Enrolled || enrolled
			status.Confirmed = status.Confirmed || confirmed
		}
		return status
	}
	// One method, or both with require All: the account needs every method listed.
	status.Enrolled, status.Confirmed = true, true
	for _, m := range req.Methods {
		enrolled, confirmed := f.has(m)
		status.Enrolled = status.Enrolled && enrolled
		status.Confirmed = status.Confirmed && confirmed
	}
	return status
}
