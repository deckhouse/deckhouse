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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Dex renders a page from the set of its templates whose frontend extra "lang" is the language the browser
// prefers (Accept-Language, Deckhouse patch 350; its tests cover the choice of the set for every page). These tests
// cover the other half: every page of the theme with Russian texts shows them with "ru" and none with "en".
var _ = Describe("Module :: user-authn :: Dex pages in both languages", func() {
	// The visible text: no scripts, styles or tags.
	scripts := regexp.MustCompile(`(?s)<(script|style)[^>]*>.*?</(script|style)>`)
	tags := regexp.MustCompile(`(?s)<[^>]*>`)
	cyrillic := regexp.MustCompile(`\p{Cyrillic}`)
	latinWord := regexp.MustCompile(`\b[A-Za-z]{4,}\b`)
	visible := func(page string) string { return tags.ReplaceAllString(scripts.ReplaceAllString(page, " "), " ") }

	strs := func(keys ...string) map[string]any {
		m := map[string]any{"ReqPath": "/auth", "BuildIdentifier": "b1"}
		for _, k := range keys {
			m[k] = ""
		}
		return m
	}
	with := func(m map[string]any, kv ...any) map[string]any {
		out := map[string]any{}
		for k, v := range m {
			out[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			out[kv[i].(string)] = kv[i+1]
		}
		return out
	}

	pages := []struct {
		name string
		file string
		data map[string]any
	}{
		{"approval", "approval.html", with(strs("AuthReqID", "Client"), "Scopes", []string{"openid"})},
		{"device", "device.html", with(strs("PostURL", "UserCode"), "Invalid", true)},
		{"device success", "device_success.html", strs("ClientName")},
		{"expired sign-in", "error.html", with(strs("ErrType", "ErrMsg"), "ErrCode", "login_expired", "ReturnURL", "https://app.example.com/")},
		{"misconfigured second factor", "error.html", with(strs("ErrType", "ErrMsg", "ReturnURL"), "ErrCode", "mfa_misconfigured")},
		{"home", "home.html", with(strs("ConnectorName", "DiscoveryURL", "Email", "IPAddress", "LogoutURL", "SessionExpiresISO",
			"SessionExpiresText", "SignedInISO", "SignedInText", "UserAgent", "Username"),
			"LoggedIn", true, "EmailVerified", true, "SessionExpiryIsIdle", false, "Groups", []string{})},
		{"login", "login.html", with(strs(), "Connectors", []map[string]string{})},
		{"logout", "logout.html", with(strs("BackURL", "ClientID", "PostLogoutRedirectURI", "State"), "LoggedOut", true, "ShowConfirmation", false)},
		{"logout confirmation", "logout.html", with(strs("BackURL", "ClientID", "PostLogoutRedirectURI", "State"), "LoggedOut", false, "ShowConfirmation", true)},
		{"mfa choose, registration", "mfa_choose.html", with(strs(), "Enroll", true,
			"Choices", []map[string]string{{"Type": "TOTP", "URL": "/mfa/totp"}, {"Type": "WebAuthn", "URL": "/mfa/webauthn"}})},
		{"mfa choose, registered", "mfa_choose.html", with(strs(), "Enroll", false,
			"Choices", []map[string]string{{"Type": "TOTP", "URL": "/mfa/totp"}})},
		{"oob", "oob.html", strs("Code")},
		{"password", "password.html", with(strs("BackLink", "PostURL", "Username", "UsernamePrompt"),
			"Invalid", true, "ShowRememberMe", true, "RememberMeChecked", false)},
		// The hint and the refusal are the English texts of the password policy of Dex (patch 220).
		{"password change", "password_change.html", with(strs("Username"), "Reason", "forced",
			"PasswordPolicy", map[string]any{"ComplexityRequirements": "Password must contain:\n• At least 12 characters\n" +
				"• Upper and lowercase letters\n• At least one number\n• Special characters (!@#$%^&* etc.)\n" +
				"• No more than 2 identical characters in a row"},
			"Error", map[string]any{"Exists": true, "List": map[string]any{"PasswordTooWeak": map[string]any{
				"Exists": true, "Description": "minimum 12 characters required"}}})},
		{"password change, a missing number", "password_change.html", with(strs("Username"), "Reason", "complexity",
			"PasswordPolicy", map[string]any{"ComplexityRequirements": "Password must contain:\n• At least one lowercase letter"},
			"Error", map[string]any{"Exists": true, "List": map[string]any{"PasswordTooWeak": map[string]any{
				"Exists": true, "Description": "at least one number required"}}})},
		{"error", "error.html", with(strs("ReturnURL", "ErrCode"), "ErrType", "Unauthorized", "ErrMsg", "Unauthorized request.")},
		{"totp, registration", "totp_verify.html", with(strs("Connector", "Issuer", "PostURL", "OtherMethodURL"), "QRCode", "aGVsbG8=", "Invalid", true)},
		{"totp, with another method", "totp_verify.html", with(strs("Connector", "Issuer", "PostURL", "QRCode"), "OtherMethodURL", "/mfa?choose=1", "Invalid", false)},
		{"webauthn, registration", "webauthn_verify.html", with(strs("OtherMethodURL"), "Mode", "register")},
		{"webauthn, with another method", "webauthn_verify.html", with(strs(), "Mode", "login", "OtherMethodURL", "/mfa?choose=1")},
	}

	It("covers every page of the theme with Russian texts", func() {
		files, err := os.ReadDir(dexTemplatesDir)
		Expect(err).NotTo(HaveOccurred())
		covered := map[string]bool{"header.html": true, "footer.html": true} // rendered with every page
		for _, p := range pages {
			covered[p.file] = true
		}
		for _, f := range files {
			content, err := os.ReadFile(dexTemplatesDir + "/" + f.Name())
			Expect(err).NotTo(HaveOccurred())
			if strings.Contains(string(content), `extra "lang"`) {
				Expect(covered).To(HaveKey(f.Name()), "a page of the theme with Russian texts and no test of its languages")
			}
		}
	})

	for _, p := range pages {
		It("renders "+p.name+" in Russian and in English", func() {
			ruPage, enPage := renderDexPage(p.file, "ru", p.data), renderDexPage(p.file, "en", p.data)
			ru, en := visible(ruPage), visible(enPage)
			Expect(ru).To(MatchRegexp(cyrillic.String()), "no Russian text")
			Expect(en).NotTo(MatchRegexp(cyrillic.String()), "Russian text on the English page")
			Expect(ru).NotTo(Equal(en))
			Expect(latinWord.MatchString(en)).To(BeTrue())
			// The page names its language for the browser and the screen readers.
			Expect(ruPage).To(ContainSubstring(`<html lang="ru">`))
			Expect(enPage).To(ContainSubstring(`<html lang="en">`))
			// No English sentence of Dex is left on the Russian page: the messages are translated.
			Expect(ru).NotTo(MatchRegexp(`(?i)\b(must|least|required|letters?|characters?|request)\b`))
		})
	}

	// The fixed messages of an error page of the patched Dex have a Russian text: the messages of upstream Dex in
	// upstreamDexErrorMessages, a list kept by hand, and the ones the patches add. A message that reaches the page
	// through a variable is checked only when it is in that list or comes from a call the scan of the patches
	// knows; a message Dex formats at run time has no fixed key and stays English.
	It("translates every fixed message of the error pages", func() {
		messages, err := dexErrorMessages(dexPatchesDir)
		Expect(err).NotTo(HaveOccurred())
		for _, msg := range messages {
			ru := visible(renderDexPage("error.html", "ru", with(strs("ReturnURL", "ErrCode"), "ErrType", "Bad Request", "ErrMsg", msg)))
			Expect(ru).NotTo(ContainSubstring(msg), "no Russian text for %q in messages.html", msg)
		}
	})

	It("finds the messages of the error page calls a patch adds", func() {
		dir, err := os.MkdirTemp("", "dex-patches")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(dir)
		patch := strings.Join([]string{
			`+const errMsgSample = "Constant message."`,
			`+		h.renderError(r, w, http.StatusBadRequest, "Literal message.")`,
			`+		h.renderError(r, w, http.StatusBadRequest, errMsgSample)`,
			`+		return nil, "", newDisplayedErr(http.StatusBadRequest, "Displayed message.")`,
			`+		return nil, "", newDisplayedErr(http.StatusBadRequest, "Formatted %s.", name)`,
			`+		return "", &deviceFlowError{status: http.StatusBadRequest, message: "Device message."}`,
			`+		return "", &deviceFlowError{status: http.StatusUnauthorized, code: oauth2.InvalidClient, message: "JSON message."}`,
			` 		h.renderError(r, w, http.StatusBadRequest, "Context message.")`,
		}, "\n")
		Expect(os.WriteFile(filepath.Join(dir, "100-sample.patch"), []byte(patch), 0o600)).To(Succeed())

		messages, err := dexErrorMessages(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(messages).To(ContainElements("Constant message.", "Literal message.", "Displayed message.", "Device message."))
		Expect(messages).NotTo(ContainElement("Formatted %s."))
		Expect(messages).NotTo(ContainElement("JSON message."))
		Expect(messages).NotTo(ContainElement("Context message."))
	})

	// The consent page lists what the application asks for by the descriptions of the scopes Dex knows
	// (scopeDescriptions of upstream Dex v2.46.0).
	It("translates the scopes of the consent page", func() {
		scopes := []string{"Have offline access", "View basic profile information", "View your email address", "View your groups"}
		data := with(strs("AuthReqID", "Client"), "Scopes", scopes)
		ru, en := visible(renderDexPage("approval.html", "ru", data)), visible(renderDexPage("approval.html", "en", data))
		for _, scope := range scopes {
			Expect(ru).NotTo(ContainSubstring(scope), "no Russian text for %q in messages.html", scope)
			Expect(en).To(ContainSubstring(scope))
		}
	})
})

const dexPatchesDir = "../images/dex/patches"

// upstreamDexErrorMessages are the fixed messages that upstream Dex v2.46.0 renders on its error page, less the ones
// the patches remove: the literals and constants of its renderError calls, the authorization request errors of
// newDisplayedErr and the HTML errors of the device flow (deviceFlowError without an OAuth2 code). The list is kept
// by hand, the test does not read the source of Dex: update it with Dex.
var upstreamDexErrorMessages = []string{
	"Approval rejected.",
	"Authentication failed. Please contact your administrator or try again later.",
	"Authorization failed. Please try again.",
	"Bad connector ID",
	"Connector ID does not match a valid Connector",
	"Connector Login Error",
	"Connector failed to initialize",
	"Connector failed to initialize.",
	"Connector not allowed for this client.",
	"Database error.",
	"Failed to connect to the database.",
	"Failed to exchange auth code.",
	"Failed to parse request.",
	"Failed to retrieve client.",
	"Failed to retrieve connector list.",
	"Handling the / path error.",
	"Health check failed.",
	"Internal server error. Please contact your administrator or try again later.",
	"Internal server error.",
	"Invalid client.",
	"Invalid client_id provided.",
	"Invalid client_id.",
	"Invalid device code request type",
	"Invalid id_token_hint.",
	"Invalid or expired auth code.",
	"Invalid or expired device code.",
	"Invalid or expired user code.",
	"Invalid redirect URI.",
	"Invalid request",
	"Invalid request. Please try again.",
	"Invalid session cookie.",
	"Invalid session.",
	"Login error. Please contact your administrator or try again later.",
	"Login error.",
	"Login process not yet finalized.",
	"MFA not enrolled.",
	"Method not allowed.",
	"Method not supported",
	"No connectors available for this client.",
	"No logout in progress.",
	"No user code received",
	"Not a TOTP authenticator.",
	"Page not found",
	"Request was missing parameters",
	"Requested connector does not support this grant type.",
	"Requested resource does not exist",
	"Requested resource does not exist.",
	"Session not found.",
	"Unauthorized request",
	"Unauthorized request.",
	"Unknown authenticator.",
	"Unregistered post_logout_redirect_uri.",
	"Unregistered redirect_uri.",
	"Unsupported request method.",
	"User session error.",
	"You are not a member of any of the required groups to authenticate.",
}

// upstreamDexErrorConstants are the constants of upstream Dex that the renderError calls of the patches pass.
var upstreamDexErrorConstants = map[string]string{
	"ErrMsgLoginError": "Login error. Please contact your administrator or try again later.",
}

var (
	// errorPageCalls are the calls whose message goes to the error page, the message in the first group: a
	// renderError call, an authorization request error and an HTML error of the device flow (one without an OAuth2
	// code, which the flow writes as JSON).
	errorPageCalls = []*regexp.Regexp{
		regexp.MustCompile(`renderError\(\s*\w+,\s*\w+,\s*[^,]+,\s*("(?:[^"\\]|\\.)*"|[\w.]+)\s*\)`),
		regexp.MustCompile(`newDisplayedErr\(\s*[^,()]+,\s*("(?:[^"\\]|\\.)*"|[\w.]+)\s*\)`),
		regexp.MustCompile(`deviceFlowError\{(?:[^}]*,)?\s*message:\s*("(?:[^"\\]|\\.)+"|[\w.]+)\s*\}`),
	}
	stringConstant = regexp.MustCompile(`^\+\s*(?:const\s+)?(\w+)\s*(?:string\s*)?=\s*"((?:[^"\\]|\\.)*)"`)
)

// dexErrorMessages returns the upstream messages and the ones of the calls the patches add (errorPageCalls), their
// constants resolved. A call a later patch replaces still counts: its message stays in the dictionary as well. A
// call that spans lines is not seen.
func dexErrorMessages(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.patch"))
	if err != nil {
		return nil, err
	}
	added := map[string]bool{}
	constants := map[string]string{}
	for name, text := range upstreamDexErrorConstants {
		constants[name] = text
	}
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(content), "\n") {
			if !strings.HasPrefix(line, "+") {
				continue
			}
			if m := stringConstant.FindStringSubmatch(line); m != nil {
				constants[m[1]] = m[2]
			}
			for _, re := range errorPageCalls {
				for _, call := range re.FindAllStringSubmatch(line, -1) {
					if !strings.Contains(call[0], "code:") {
						added[call[1]] = true
					}
				}
			}
		}
	}

	messages := append([]string{}, upstreamDexErrorMessages...)
	for arg := range added {
		if strings.HasPrefix(arg, `"`) {
			msg, err := strconv.Unquote(arg)
			if err != nil {
				return nil, err
			}
			messages = append(messages, msg)
			continue
		}
		msg, ok := constants[arg[strings.LastIndex(arg, ".")+1:]]
		if !ok {
			return nil, fmt.Errorf("an error page call of the patches passes %s, a constant the test does not know", arg)
		}
		messages = append(messages, msg)
	}
	sort.Strings(messages)
	return slices.Compact(messages), nil
}
