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

package template_tests

import (
	"bytes"
	"html/template"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// renderDexPage renders a page of the Dex theme with the functions of dex it uses (server/templates/templates.go,
// getFuncMap) for a frontend.extra.lang of lang.
func renderDexPage(page, lang string, data any) string {
	t := template.New("").Funcs(template.FuncMap{
		"extra": func(k string) string { return map[string]string{"lang": lang}[k] },
		"url":   func(_, asset string) string { return asset },
	})
	t, err := t.ParseFiles(
		filepath.Join(dexTemplatesDir, "header.html"),
		filepath.Join(dexTemplatesDir, "footer.html"),
		filepath.Join(dexTemplatesDir, page),
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var out bytes.Buffer
	ExpectWithOffset(1, t.ExecuteTemplate(&out, page, data)).To(Succeed())
	return out.String()
}

// dexMFAChoice and dexMFAChooseData are the data dex gives mfa_choose.html (Deckhouse patch 340,
// server/templates/templates.go, MFAChoose).
type dexMFAChoice struct {
	Type string
	URL  string
}

type dexMFAChooseData struct {
	Choices         []dexMFAChoice
	Enroll          bool
	ReqPath         string
	BuildIdentifier string
}

var _ = Describe("Module :: user-authn :: Dex second factor pages", func() {
	choices := []dexMFAChoice{
		{Type: "TOTP", URL: "https://dex.example.com/mfa/totp?authenticator=totp"},
		{Type: "WebAuthn", URL: "https://dex.example.com/mfa/webauthn?authenticator=webauthn"},
	}
	text := regexp.MustCompile(`(?s)<div class="content-text">\s*(.*?)\s*</div>`)
	// The no-cyrillic checks keep Cyrillic out of the Go sources, so the Russian texts are checked for being
	// Russian.
	russian := regexp.MustCompile(`\p{Cyrillic}`)

	It("offers each method by its address, in the order given", func() {
		page := renderDexPage("mfa_choose.html", "en", dexMFAChooseData{Choices: choices, Enroll: true})
		Expect(page).To(ContainSubstring(`id="choice-TOTP"`))
		Expect(page).To(ContainSubstring(`href="https://dex.example.com/mfa/totp?authenticator=totp"`))
		Expect(page).To(ContainSubstring("Authenticator app"))
		Expect(page).To(ContainSubstring("Passkey or security key"))
		Expect(bytes.Index([]byte(page), []byte("choice-TOTP"))).To(BeNumerically("<", bytes.Index([]byte(page), []byte("choice-WebAuthn"))))
	})

	It("tells a registration from a choice of a registered method", func() {
		Expect(text.FindStringSubmatch(renderDexPage("mfa_choose.html", "en", dexMFAChooseData{Choices: choices, Enroll: true}))[1]).
			To(Equal("Choose how you will confirm your sign-ins."))
		Expect(text.FindStringSubmatch(renderDexPage("mfa_choose.html", "en", dexMFAChooseData{Choices: choices}))[1]).
			To(Equal("Choose how to confirm this sign-in."))
	})

	It("is in Russian", func() {
		for _, enroll := range []bool{true, false} {
			page := renderDexPage("mfa_choose.html", "ru", dexMFAChooseData{Choices: choices, Enroll: enroll})
			Expect(text.FindStringSubmatch(page)[1]).To(MatchRegexp(russian.String()))
			Expect(page).NotTo(ContainSubstring("Authenticator app"))
			Expect(page).NotTo(ContainSubstring("Passkey or security key"))
		}
	})

	for _, tc := range []struct {
		page string
		data func(other string) any
	}{
		{"totp_verify.html", func(other string) any {
			return struct {
				PostURL, Issuer, Connector, QRCode, OtherMethodURL, ReqPath, BuildIdentifier string
				Invalid                                                                     bool
			}{PostURL: "/mfa/totp", Issuer: "Deckhouse", Connector: "local", OtherMethodURL: other}
		}},
		{"webauthn_verify.html", func(other string) any {
			return struct {
				Mode, AuthenticatorID, OtherMethodURL, ReqPath, BuildIdentifier string
			}{Mode: "login", AuthenticatorID: "webauthn", OtherMethodURL: other}
		}},
	} {
		It("offers another method on "+tc.page+" only with its address", func() {
			en := renderDexPage(tc.page, "en", tc.data("https://dex.example.com/mfa?choose=1"))
			Expect(en).To(ContainSubstring(`id="other-method"`))
			Expect(en).To(ContainSubstring(`href="https://dex.example.com/mfa?choose=1"`))
			Expect(en).To(ContainSubstring("Sign in another way"))
			ru := renderDexPage(tc.page, "ru", tc.data("https://dex.example.com/mfa?choose=1"))
			Expect(ru).To(ContainSubstring(`id="other-method"`))
			Expect(ru).NotTo(ContainSubstring("Sign in another way"))
			Expect(renderDexPage(tc.page, "en", tc.data(""))).NotTo(ContainSubstring(`id="other-method"`))
		})
	}
})
