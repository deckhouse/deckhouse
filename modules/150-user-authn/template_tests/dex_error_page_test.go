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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const dexTemplatesDir = "../images/dex/src/web/templates"

// dexErrorPageData is the data dex gives error.html, with ErrCode and ReturnURL of patch 280
// (server/templates/templates.go, renderErr).
type dexErrorPageData struct {
	ErrType         string
	ErrMsg          string
	ErrCode         string
	ReturnURL       string
	ReqPath         string
	BuildIdentifier string
}

// loadDexErrorPage parses the error page of the theme with the functions of dex it uses
// (server/templates/templates.go, getFuncMap) for a frontend.extra.lang of lang.
func loadDexErrorPage(lang string) *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"extra": func(k string) string { return map[string]string{"lang": lang}[k] },
		"url":   func(_, asset string) string { return asset },
	})
	t, err := t.ParseFiles(
		filepath.Join(dexTemplatesDir, "header.html"),
		filepath.Join(dexTemplatesDir, "footer.html"),
		filepath.Join(dexTemplatesDir, "error.html"),
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return t
}

func renderDexErrorPage(lang string, data dexErrorPageData) string {
	var out bytes.Buffer
	ExpectWithOffset(1, loadDexErrorPage(lang).ExecuteTemplate(&out, "error.html", data)).To(Succeed())
	return out.String()
}

var _ = Describe("Module :: user-authn :: Dex error page", func() {
	expired := dexErrorPageData{
		ErrType:   "Bad Request",
		ErrMsg:    "The sign-in has expired. Go back to the application and sign in again.",
		ErrCode:   "login_expired",
		ReturnURL: "https://app.example.com/",
		ReqPath:   "/auth/local/login",
	}
	heading := regexp.MustCompile(`(?s)<h2 class="content-title">\s*(.*?)\s*</h2>`)
	button := regexp.MustCompile(`(?s)<a class="btn btn-primary" href="([^"]*)">\s*(.*?)\s*</a>`)
	// The no-cyrillic checks keep Cyrillic out of the Go sources, so the Russian texts are checked for
	// being Russian and for differing from the English ones.
	russian := regexp.MustCompile(`^[\p{Cyrillic} .-]+$`)

	It("explains an expired sign-in in English", func() {
		page := renderDexErrorPage("en", expired)
		Expect(heading.FindStringSubmatch(page)[1]).To(Equal("The sign-in has expired"))
		Expect(page).To(ContainSubstring("Go back to the application and sign in again."))
		Expect(page).NotTo(ContainSubstring("Bad Request"))
		b := button.FindStringSubmatch(page)
		Expect(b).To(HaveLen(3))
		Expect(b[1]).To(Equal("https://app.example.com/"))
		Expect(b[2]).To(Equal("Return to the application"))
	})

	It("explains an expired sign-in in Russian", func() {
		en, ru := renderDexErrorPage("en", expired), renderDexErrorPage("ru", expired)
		Expect(heading.FindStringSubmatch(ru)[1]).To(MatchRegexp(russian.String()))
		Expect(heading.FindStringSubmatch(ru)[1]).NotTo(Equal(heading.FindStringSubmatch(en)[1]))
		Expect(ru).NotTo(ContainSubstring("Go back to the application"))
		b := button.FindStringSubmatch(ru)
		Expect(b).To(HaveLen(3))
		Expect(b[1]).To(Equal("https://app.example.com/"))
		Expect(b[2]).To(MatchRegexp(russian.String()))
	})

	It("offers to return only with an application to return to", func() {
		data := expired
		data.ReturnURL = ""
		for _, lang := range []string{"en", "ru"} {
			page := renderDexErrorPage(lang, data)
			Expect(page).NotTo(ContainSubstring(`class="btn btn-primary"`), lang)
			Expect(heading.FindStringSubmatch(page)).To(HaveLen(2), lang)
		}
	})

	// Dex checks the address against the client; the template still escapes it.
	It("escapes the address of the application", func() {
		data := expired
		data.ReturnURL = `javascript:alert(1)`
		Expect(renderDexErrorPage("en", data)).NotTo(ContainSubstring("javascript:"))
	})

	It("keeps the other errors as they are", func() {
		for _, lang := range []string{"en", "ru"} {
			page := renderDexErrorPage(lang, dexErrorPageData{ErrType: "Bad Request", ErrMsg: "User session error.", ReqPath: "/auth"})
			Expect(heading.FindStringSubmatch(page)[1]).To(Equal("Bad Request"), lang)
			Expect(page).To(ContainSubstring(`<div class="content-text">User session error.</div>`), lang)
			Expect(strings.Contains(page, "The sign-in has expired")).To(BeFalse(), lang)
		}
	})
})
