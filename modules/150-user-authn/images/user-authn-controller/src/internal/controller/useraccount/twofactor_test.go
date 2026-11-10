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
	"reflect"
	"testing"

	"user-authn-controller/api/v1alpha1"
)

func TestParseTwoFactorPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    TwoFactorPolicy
		wantErr bool
	}{
		{name: "2FA off", value: "", want: nil},
		{
			name:  "a method for each type",
			value: `{"Local": {"methods": ["TOTP", "WebAuthn"], "require": "All"}, "LDAP": {"methods": ["WebAuthn"]}}`,
			want: TwoFactorPolicy{
				"Local": {Methods: []string{"TOTP", "WebAuthn"}, Require: "All"},
				"LDAP":  {Methods: []string{"WebAuthn"}},
			},
		},
		{name: "not JSON", value: "TOTP", wantErr: true},
		{name: "no method", value: `{"Local": {"methods": []}}`, wantErr: true},
		{name: "an unknown method", value: `{"Local": {"methods": ["SMS"]}}`, wantErr: true},
		{name: "a method twice", value: `{"Local": {"methods": ["TOTP", "TOTP"], "require": "All"}}`, wantErr: true},
		{name: "two methods without require", value: `{"Local": {"methods": ["TOTP", "WebAuthn"]}}`, wantErr: true},
		{name: "two methods with an unknown require", value: `{"Local": {"methods": ["TOTP", "WebAuthn"], "require": "Most"}}`, wantErr: true},
		{
			name:  "a choice of one method",
			value: `{"Local": {"methods": ["TOTP", "WebAuthn"], "require": "Any"}}`,
			want:  TwoFactorPolicy{"Local": {Methods: []string{"TOTP", "WebAuthn"}, Require: "Any"}},
		},
		{name: "require with one method", value: `{"Local": {"methods": ["TOTP"], "require": "All"}}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTwoFactorPolicy(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("policy = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseMaxCredentials(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		value   string
		want    int
		wantErr bool
	}{
		{value: "", want: 0},
		{value: "3", want: 3},
		{value: "0", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "three", wantErr: true},
	} {
		got, err := ParseMaxCredentials(tc.value)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseMaxCredentials(%q) = %d, %v; want %d, error %v", tc.value, got, err, tc.want, tc.wantErr)
		}
	}
}

// The block of the keys shows the limit with the number of keys.
func TestTwoFactorStatusLimitOfKeys(t *testing.T) {
	t.Parallel()

	req := &TwoFactorRequirement{Methods: []string{"WebAuthn"}, MaxCredentials: 3}
	got := twoFactorStatus(storedFactors{WebAuthnKeys: 2}, req)
	want := &v1alpha1.UserAccountWebAuthn{Credentials: 2, MaxCredentials: 3}
	if got == nil || !reflect.DeepEqual(got.WebAuthn, want) {
		t.Errorf("webAuthn = %+v, want %+v", got, want)
	}
}

func TestTwoFactorPolicyFor(t *testing.T) {
	t.Parallel()

	policy := TwoFactorPolicy{"LDAP": {Methods: []string{"WebAuthn"}}}
	if got := policy.For("LDAP"); got == nil || !reflect.DeepEqual(got.Methods, []string{"WebAuthn"}) {
		t.Errorf("For(LDAP) = %+v", got)
	}
	// A type the settings do not cover, and no settings at all, have no requirement.
	if got := policy.For(ProviderTypeLocal); got != nil {
		t.Errorf("For(Local) = %+v, want nil", got)
	}
	if got := TwoFactorPolicy(nil).For("LDAP"); got != nil {
		t.Errorf("nil policy For(LDAP) = %+v, want nil", got)
	}
}

func TestTwoFactorStatus(t *testing.T) {
	t.Parallel()

	totp := &TwoFactorRequirement{Methods: []string{"TOTP"}}
	webAuthn := &TwoFactorRequirement{Methods: []string{"WebAuthn"}}
	both := &TwoFactorRequirement{Methods: []string{"TOTP", "WebAuthn"}, Require: "All"}
	either := &TwoFactorRequirement{Methods: []string{"TOTP", "WebAuthn"}, Require: "Any"}
	confirmedSecret := &v1alpha1.UserAccountTOTP{Confirmed: true}
	oneKey := &v1alpha1.UserAccountWebAuthn{Credentials: 1}

	tests := []struct {
		name    string
		factors storedFactors
		req     *TwoFactorRequirement
		want    v1alpha1.UserAccountTwoFactor
	}{
		{
			name:    "TOTP, a secret not confirmed yet",
			factors: storedFactors{TOTP: true},
			req:     totp,
			want:    v1alpha1.UserAccountTwoFactor{Methods: []string{"TOTP"}, Enrolled: true, TOTP: &v1alpha1.UserAccountTOTP{}},
		},
		{
			name:    "TOTP, a key does not count",
			factors: storedFactors{WebAuthnKeys: 1},
			req:     totp,
			want:    v1alpha1.UserAccountTwoFactor{Methods: []string{"TOTP"}, WebAuthn: oneKey},
		},
		{
			name:    "WebAuthn, a confirmed secret does not count",
			factors: storedFactors{TOTP: true, TOTPConfirmed: true},
			req:     webAuthn,
			want:    v1alpha1.UserAccountTwoFactor{Methods: []string{"WebAuthn"}, TOTP: confirmedSecret},
		},
		{
			name:    "both required, both there",
			factors: storedFactors{TOTP: true, TOTPConfirmed: true, WebAuthnKeys: 1},
			req:     both,
			want: v1alpha1.UserAccountTwoFactor{
				Methods: []string{"TOTP", "WebAuthn"}, Require: "All", Enrolled: true, Confirmed: true,
				TOTP: confirmedSecret, WebAuthn: oneKey,
			},
		},
		{
			name:    "both required, only the key",
			factors: storedFactors{WebAuthnKeys: 1},
			req:     both,
			want:    v1alpha1.UserAccountTwoFactor{Methods: []string{"TOTP", "WebAuthn"}, Require: "All", WebAuthn: oneKey},
		},
		{
			name:    "either, only the key",
			factors: storedFactors{WebAuthnKeys: 1},
			req:     either,
			want: v1alpha1.UserAccountTwoFactor{
				Methods: []string{"TOTP", "WebAuthn"}, Require: "Any", Enrolled: true, Confirmed: true, WebAuthn: oneKey,
			},
		},
		{
			name:    "either, a secret not confirmed",
			factors: storedFactors{TOTP: true},
			req:     either,
			want: v1alpha1.UserAccountTwoFactor{
				Methods: []string{"TOTP", "WebAuthn"}, Require: "Any", Enrolled: true, TOTP: &v1alpha1.UserAccountTOTP{},
			},
		},
		{
			name: "either, nothing",
			req:  either,
			want: v1alpha1.UserAccountTwoFactor{Methods: []string{"TOTP", "WebAuthn"}, Require: "Any"},
		},
		{
			name:    "both required, the secret not confirmed",
			factors: storedFactors{TOTP: true, WebAuthnKeys: 1},
			req:     both,
			want: v1alpha1.UserAccountTwoFactor{
				Methods: []string{"TOTP", "WebAuthn"}, Require: "All", Enrolled: true,
				TOTP: &v1alpha1.UserAccountTOTP{}, WebAuthn: oneKey,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := twoFactorStatus(tt.factors, tt.req)
			if got == nil || !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("twoFactor = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// A type of provider the second factor does not cover has no status.twoFactor, whatever Dex keeps of an earlier
// second factor of the account.
func TestTwoFactorStatusOfAnUncoveredType(t *testing.T) {
	t.Parallel()

	for _, f := range []storedFactors{{}, {TOTP: true, TOTPConfirmed: true}, {WebAuthnKeys: 2}} {
		if got := twoFactorStatus(f, nil); got != nil {
			t.Errorf("factors %+v: twoFactor = %+v, want none", f, got)
		}
	}
}
