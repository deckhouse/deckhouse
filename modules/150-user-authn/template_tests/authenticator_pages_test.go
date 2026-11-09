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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	pagesTemplatesDir = "../images/dex-authenticator/templates"
	dexStaticDir      = "../images/dex/src/web/static"
	pagesWerfFile     = "../images/dex-authenticator/werf.inc.yaml"
	pagesProxyPrefix  = "/dex-authenticator"
)

// errorPageData is the data oauth2-proxy gives error.html, with Lang and LogoData of patch 015
// (pkg/app/pagewriter/error_page.go).
type errorPageData struct {
	Title       string
	Message     string
	ProxyPrefix string
	StatusCode  int
	Redirect    string
	RequestID   string
	Footer      template.HTML
	Version     string
	Lang        string
	LogoData    template.HTML
}

// signInPageData is the data oauth2-proxy gives sign_in.html, with Lang of patch 015
// (pkg/app/pagewriter/sign_in_page.go).
type signInPageData struct {
	ProviderName  string
	SignInMessage template.HTML
	StatusCode    int
	CustomLogin   bool
	Redirect      string
	Version       string
	ProxyPrefix   string
	Footer        template.HTML
	LogoData      template.HTML
	Lang          string
}

// loadPagesTemplates parses the templates the way oauth2-proxy does (pkg/app/pagewriter/templates.go).
func loadPagesTemplates() *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"ToUpper":   strings.ToUpper,
		"ToLower":   strings.ToLower,
		"HasPrefix": strings.HasPrefix,
	})
	t, err := t.ParseFiles(filepath.Join(pagesTemplatesDir, "sign_in.html"), filepath.Join(pagesTemplatesDir, "error.html"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return t
}

func renderPage(t *template.Template, name string, data any) string {
	var out bytes.Buffer
	ExpectWithOffset(1, t.ExecuteTemplate(&out, name, data)).To(Succeed())
	return out.String()
}

func errorPage(lang string, status int, message string) errorPageData {
	return errorPageData{
		Title:       "Forbidden",
		Message:     message,
		ProxyPrefix: pagesProxyPrefix,
		StatusCode:  status,
		Redirect:    "/app/page",
		RequestID:   "request-1",
		Lang:        lang,
		LogoData:    `<svg id="deckhouse-logo"></svg>`,
	}
}

var (
	resourceAttr = regexp.MustCompile(`<(link|img|source|script|iframe)\b[^>]*?\b(href|src|srcset)="([^"]*)"`)
	cssURL       = regexp.MustCompile(`url\("?([^")]+)"?\)`)
)

// pageResources lists the addresses of the resources a page loads.
func pageResources(page string) []string {
	var resources []string
	for _, m := range resourceAttr.FindAllStringSubmatch(page, -1) {
		resources = append(resources, m[3])
	}
	return resources
}

// werfStaticIncludes lists the includePaths of the Dex static files in the image of the authenticator.
func werfStaticIncludes() []string {
	werf, err := os.ReadFile(pagesWerfFile)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	block := regexp.MustCompile(`(?s)images/dex/src/web/static\n\s+to: /dex-static\n\s+includePaths:\n(.*?)\n\s+stageDependencies:`).FindSubmatch(werf)
	ExpectWithOffset(1, block).NotTo(BeNil(), "the Dex static files are not added to the image")
	var includes []string
	for _, line := range strings.Split(string(block[1]), "\n") {
		includes = append(includes, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- ")))
	}
	return includes
}

// expectInImage checks that a file of the Dex static directory exists and gets into the image.
func expectInImage(file string, includes []string) {
	_, err := os.Stat(filepath.Join(dexStaticDir, file))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), file)
	matched := false
	for _, include := range includes {
		if ok, _ := path.Match(include, file); ok {
			matched = true
		}
	}
	ExpectWithOffset(1, matched).To(BeTrue(), "%s is not in the includePaths of the image", file)
}

var _ = Describe("Module :: user-authn :: DexAuthenticator pages", func() {
	t := loadPagesTemplates()

	// The English texts are checked word for word. The Russian ones only for being Russian and for telling the
	// same cases apart as the English ones do: the no-cyrillic checks keep Cyrillic out of the Go sources.
	cases := []struct {
		status        int
		message       string
		title, button string
	}{
		{403, "Login Failed: The sign-in has expired. Unable to find a valid CSRF token. Please sign in again.", "The sign-in has expired", "Sign in again"},
		{403, "Login Failed: Unable to find a valid CSRF token. Please try again.", "Sign-in did not complete", "Sign in again"},
		{403, "Login Failed: The upstream identity provider returned an error: access_denied", "Sign-in refused", "Sign in again"},
		{403, "You do not have permission to access this resource.", "Access denied", "Sign in again"},
		{401, "You need to be logged in to access this resource.", "Sign-in required", "Sign in again"},
		{503, "The session cannot be checked now, try again later", "Sign-in is temporarily unavailable", "Sign in again"},
		{500, "Oops! Something went wrong.", "Something went wrong", "Sign in again"},
		{502, "There was a problem connecting to the upstream server.", "Something went wrong", "Sign in again"},
		{404, "We could not find the resource you were looking for.", "Page not found", "Back to application"},
	}
	heading := regexp.MustCompile(`<h2 class="content-title">([^<]*)</h2>`)
	action := regexp.MustCompile(`class="btn btn-primary(?:-outline)?">([^<]*)<`)
	russian := regexp.MustCompile(`^[\p{Cyrillic} -]+$`)

	It("explains the failure in English", func() {
		for _, tc := range cases {
			page := renderPage(t, "error.html", errorPage("en", tc.status, tc.message))
			Expect(page).To(ContainSubstring(`<html lang="en">`), tc.message)
			Expect(page).To(ContainSubstring(`<h2 class="content-title">`+tc.title+`</h2>`), tc.message)
			Expect(page).To(ContainSubstring(`<title>`+tc.title+`</title>`), tc.message)
			Expect(action.FindStringSubmatch(page)).To(HaveLen(2), tc.message)
			Expect(action.FindStringSubmatch(page)[1]).To(Equal(tc.button), tc.message)
		}
	})

	It("explains the failure in Russian for the same cases", func() {
		ruOf := map[string]string{}
		for _, tc := range cases {
			page := renderPage(t, "error.html", errorPage("ru", tc.status, tc.message))
			Expect(page).To(ContainSubstring(`<html lang="ru">`), tc.message)
			title := heading.FindStringSubmatch(page)
			Expect(title).To(HaveLen(2), tc.message)
			Expect(title[1]).To(MatchRegexp(russian.String()), tc.message)
			Expect(page).To(ContainSubstring(`<title>`+title[1]+`</title>`), tc.message)
			button := action.FindStringSubmatch(page)
			Expect(button).To(HaveLen(2), tc.message)
			Expect(button[1]).To(MatchRegexp(russian.String()), tc.message)
			// One Russian title per English one, and the other way round.
			if prev, ok := ruOf[tc.title]; ok {
				Expect(title[1]).To(Equal(prev), tc.message)
			}
			ruOf[tc.title] = title[1]
		}
		seen := map[string]bool{}
		for _, ru := range ruOf {
			Expect(seen[ru]).To(BeFalse(), ru)
			seen[ru] = true
		}
	})

	It("signs in again to where the user was going", func() {
		page := renderPage(t, "error.html", errorPage("en", 403, "Login Failed: Unable to find a valid CSRF token. Please try again."))
		Expect(page).To(ContainSubstring(`<form method="GET" action="/dex-authenticator/sign_in" class="login-form">`))
		Expect(page).To(ContainSubstring(`<input type="hidden" name="rd" value="/app/page">`))
	})

	It("does not offer to sign in on a missing page", func() {
		page := renderPage(t, "error.html", errorPage("en", 404, ""))
		Expect(page).NotTo(ContainSubstring(`/dex-authenticator/sign_in`))
		Expect(page).To(ContainSubstring(`<a href="/app/page" class="btn btn-primary-outline">`))
	})

	It("keeps the message, the status and the request ID in the details", func() {
		page := renderPage(t, "error.html", errorPage("en", 403, "Login Failed: Unable to find a valid CSRF token. Please try again."))
		Expect(page).To(ContainSubstring(`<summary class="content-text">Details</summary>`))
		Expect(page).To(ContainSubstring(`<dd>403</dd>`))
		Expect(page).To(ContainSubstring(`Login Failed: Unable to find a valid CSRF token. Please try again.`))
		Expect(page).To(ContainSubstring(`request-1`))
	})

	It("escapes the message and the redirect", func() {
		data := errorPage("en", 500, `<script>alert(1)</script>`)
		data.Redirect = `/"><script>alert(2)</script>`
		page := renderPage(t, "error.html", data)
		Expect(page).NotTo(ContainSubstring(`<script>`))
	})

	It("shows the logo only when the proxy has one", func() {
		Expect(renderPage(t, "error.html", errorPage("en", 403, ""))).
			To(ContainSubstring(`<div class="layout-deckhouse-title"><svg id="deckhouse-logo"></svg></div>`))

		data := errorPage("en", 403, "")
		data.LogoData = ""
		Expect(renderPage(t, "error.html", data)).NotTo(ContainSubstring(`layout-deckhouse-title`))
	})

	It("falls back to English for a page without a language", func() {
		page := renderPage(t, "error.html", errorPage("", 403, "You do not have permission to access this resource."))
		Expect(page).To(ContainSubstring(`<html lang="en">`))
		Expect(page).To(ContainSubstring(`Access denied`))
	})

	It("renders the sign-in page in the language of the browser", func() {
		page := renderPage(t, "sign_in.html", signInPageData{ProxyPrefix: pagesProxyPrefix, Redirect: "/app/page", Lang: "ru"})
		Expect(page).To(ContainSubstring(`<html lang="ru">`))
		Expect(page).To(ContainSubstring(`<form method="GET" action="/dex-authenticator/start" class="login-form">`))
		Expect(page).To(ContainSubstring(`<input type="hidden" name="rd" value="/app/page">`))
	})

	// The pages run no scripts and load everything from the proxy itself, so they need no exception in a
	// Content-Security-Policy, and the resources they load are the Dex files that the image gets.
	It("loads only the Dex files that the image of the authenticator carries", func() {
		includes := werfStaticIncludes()
		pages := []string{
			renderPage(t, "error.html", errorPage("en", 403, "")),
			renderPage(t, "error.html", errorPage("ru", 403, "")),
			renderPage(t, "sign_in.html", signInPageData{ProxyPrefix: pagesProxyPrefix, Lang: "en"}),
			renderPage(t, "sign_in.html", signInPageData{ProxyPrefix: pagesProxyPrefix, Lang: "ru"}),
		}
		for _, page := range pages {
			Expect(page).NotTo(ContainSubstring(`<script`))
			resources := pageResources(page)
			Expect(resources).NotTo(BeEmpty())
			for _, resource := range resources {
				Expect(resource).To(HavePrefix(pagesProxyPrefix + "/static/"))
				expectInImage(strings.TrimPrefix(resource, pagesProxyPrefix+"/static/"), includes)
			}
		}

		css, err := os.ReadFile(filepath.Join(dexStaticDir, "main.css"))
		Expect(err).NotTo(HaveOccurred())
		for _, m := range cssURL.FindAllStringSubmatch(string(css), -1) {
			// main.css is served at <prefix>/static/main.css, as in Dex at /static/main.css.
			resolved := path.Join("/static", m[1])
			Expect(resolved).To(HavePrefix("/static/"), m[1])
			expectInImage(strings.TrimPrefix(resolved, "/static/"), includes)
		}
	})
})
