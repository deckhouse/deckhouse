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

// Fuzz harnesses for the node services rendering pipeline.
//
// Threat model coverage (registry-threat-model.md):
//
//   - TM-02 / AS-02: values of `ProxyConfig` (`http`, `https`, `no_proxy`) reach the
//     static pod manifest template as `value: {{ .HTTP }}` without escaping. The
//     manifest is written to /etc/kubernetes/manifests and executed by kubelet as
//     root on a control plane node, so a value able to alter the YAML structure is
//     equivalent to arbitrary static pod creation.
//
//   - TM-08 / AS-08: values of `LocalMode.Upstreams` and of the upstream registry
//     (`Scheme`, `Host`) reach the `mirrorer` and `distribution` configuration
//     templates inside double-quoted YAML scalars without escaping.
//
// The oracle is deliberately structural: whatever the input, the rendered document
// must (a) parse as YAML and (b) decode into the expected shape with the input
// reproduced verbatim as a string. Both an added/removed mapping key and a scalar
// type change (missing `quote`) violate it.
package staticpod

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"sigs.k8s.io/yaml"
)

// ---------------------------------------------------------------------------
// Decoded shapes used as the oracle.
// ---------------------------------------------------------------------------

type podDoc struct {
	APIVersion string  `json:"apiVersion"`
	Kind       string  `json:"kind"`
	Metadata   podMeta `json:"metadata"`
	Spec       podSpec `json:"spec"`
}

type podMeta struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type podSpec struct {
	HostNetwork bool           `json:"hostNetwork"`
	Containers  []podContainer `json:"containers"`
	Volumes     []podVolume    `json:"volumes"`
}

type podContainer struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Env   []podEnv `json:"env"`
}

type podEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type podVolume struct {
	Name string `json:"name"`
}

type distributionDoc struct {
	Version string                `json:"version"`
	Log     map[string]any        `json:"log"`
	Storage map[string]any        `json:"storage"`
	HTTP    distributionHTTPDoc   `json:"http"`
	Proxy   *distributionProxyDoc `json:"proxy"`
	Auth    distributionAuthDoc   `json:"auth"`
}

type distributionHTTPDoc struct {
	Addr   string         `json:"addr"`
	Prefix string         `json:"prefix"`
	Secret string         `json:"secret"`
	Debug  map[string]any `json:"debug"`
	TLS    map[string]any `json:"tls"`
	RealIP map[string]any `json:"realip"`
}

type distributionProxyDoc struct {
	RemoteURL      string  `json:"remoteurl"`
	Username       string  `json:"username"`
	Password       string  `json:"password"`
	RemotePathOnly string  `json:"remotepathonly"`
	LocalPathAlias string  `json:"localpathalias"`
	CA             string  `json:"ca"`
	TTL            *string `json:"ttl"`
}

type distributionAuthDoc struct {
	Token distributionTokenDoc `json:"token"`
}

type distributionTokenDoc struct {
	Realm          string         `json:"realm"`
	Service        string         `json:"service"`
	Issuer         string         `json:"issuer"`
	RootCertBundle string         `json:"rootcertbundle"`
	AutoRedirect   bool           `json:"autoredirect"`
	Proxy          map[string]any `json:"proxy"`
}

type mirrorerDoc struct {
	CA     string                     `json:"ca"`
	Users  map[string]mirrorerUserDoc `json:"users"`
	Local  string                     `json:"local"`
	Remote []string                   `json:"remote"`
}

type mirrorerUserDoc struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type authDoc struct {
	Server map[string]any         `json:"server"`
	Token  map[string]any         `json:"token"`
	Users  map[string]authUserDoc `json:"users"`
	ACL    []authACLDoc           `json:"acl"`
}

type authUserDoc struct {
	Password string `json:"password"`
}

type authACLDoc struct {
	Match   map[string]string `json:"match"`
	Actions []string          `json:"actions"`
	Comment string            `json:"comment"`
}

// renderAndDecode renders the model and decodes the result into out. A render
// error, a YAML syntax error or a type mismatch all mean the substituted value
// escaped its scalar.
func renderAndDecode(t *testing.T, renderer templateRenderer, out any) []byte {
	t.Helper()

	buf, err := renderer.Render()
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if err := yaml.Unmarshal(buf, out); err != nil {
		t.Fatalf("rendered document is not the expected YAML document: %v\n---\n%s\n---", err, buf)
	}

	return buf
}

// shapeOf reduces a decoded YAML document to its structure: mapping keys and
// sequence lengths are kept, every scalar is replaced by its type name. Two
// documents with the same shape differ only in scalar values, which is exactly
// the property a correctly escaped template substitution must preserve.
func shapeOf(value any) any {
	switch v := value.(type) {
	case map[string]any:
		shape := make(map[string]any, len(v))
		for key, item := range v {
			shape[key] = shapeOf(item)
		}
		return shape
	case []any:
		shape := make([]any, 0, len(v))
		for _, item := range v {
			shape = append(shape, shapeOf(item))
		}
		return shape
	default:
		return fmt.Sprintf("%T", value)
	}
}

// assertSameShape renders both models and fails if the substituted values changed
// the document structure. reference must be the same model with every fuzzed
// value replaced by an inert placeholder of the same emptiness.
//
// dataKeyedPaths lists top-level keys of mappings whose *keys* are themselves
// derived from the fuzzed input (the auth config keys accounts by user name).
// For those the key set legitimately differs between the two renderings, so only
// the number of entries and the shape of each value are compared.
func assertSameShape(t *testing.T, fuzzed, reference templateRenderer, dataKeyedPaths ...string) {
	t.Helper()

	var got, want map[string]any

	buf := renderAndDecode(t, fuzzed, &got)
	renderAndDecode(t, reference, &want)

	for _, path := range dataKeyedPaths {
		gotEntries, gotOK := got[path].(map[string]any)
		wantEntries, wantOK := want[path].(map[string]any)
		if gotOK != wantOK {
			t.Fatalf("substituted value changed the type of %q\nrendered:\n---\n%s\n---", path, buf)
		}
		if gotOK {
			if len(gotEntries) != len(wantEntries) {
				t.Fatalf("substituted value changed the number of %q entries: got %d want %d\nrendered:\n---\n%s\n---",
					path, len(gotEntries), len(wantEntries), buf)
			}
			// Every value must keep the shape the template defines for it.
			var reference any
			for _, value := range wantEntries {
				reference = shapeOf(value)
				break
			}
			for key, value := range gotEntries {
				if shape := shapeOf(value); !reflect.DeepEqual(shape, reference) {
					t.Fatalf("entry %q of %q has an unexpected shape %v (want %v)\nrendered:\n---\n%s\n---",
						key, path, shape, reference, buf)
				}
			}
		}
		delete(got, path)
		delete(want, path)
	}

	gotShape, wantShape := shapeOf(got), shapeOf(want)
	if !reflect.DeepEqual(gotShape, wantShape) {
		t.Fatalf("substituted value altered the document structure\ngot shape:  %v\nwant shape: %v\nrendered:\n---\n%s\n---",
			gotShape, wantShape, buf)
	}
}

// requireUTF8 skips the inputs the templates deliberately refuse to render.
//
// yamlQuote rejects a value that is not valid UTF-8, because such a value has no
// YAML representation and would otherwise be silently rewritten. The renderer
// then returns an error and no file is written, which is the safe outcome. The
// harness asserts that the rejection actually happens and stops there, rather
// than reporting a refused render as a failure.
func requireUTF8(t *testing.T, renderer templateRenderer, values ...string) {
	t.Helper()

	for _, value := range values {
		if utf8.ValidString(value) {
			continue
		}

		if _, err := renderer.Render(); err == nil {
			t.Fatalf("template rendered %q, which is not valid UTF-8, instead of refusing it", value)
		}
		t.Skip("value is not valid UTF-8: the renderer refuses it")
	}
}

// inert returns a placeholder that cannot affect YAML structure, preserving
// whether the original value was empty (templates branch on emptiness).
func inert(value string) string {
	if value == "" {
		return ""
	}
	return "placeholder"
}

// inertDistinct is inert for values that become mapping keys: it preserves both
// emptiness and the equality classes of the inputs, so that keys which collapse
// into one another keep collapsing in the reference document.
func inertDistinct(values ...string) []string {
	assigned := make(map[string]string, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if value == "" {
			out = append(out, "")
			continue
		}
		if placeholder, ok := assigned[value]; ok {
			out = append(out, placeholder)
			continue
		}
		placeholder := fmt.Sprintf("placeholder-%d", len(assigned))
		assigned[value] = placeholder
		out = append(out, placeholder)
	}

	return out
}

// ---------------------------------------------------------------------------
// TM-02: ProxyConfig -> static pod manifest.
// ---------------------------------------------------------------------------

func FuzzStaticPodManifestProxyEnvs(f *testing.F) {
	// The five values that reach the pod manifest, varied one at a time. The
	// proxy settings are rendered as container environment, so a payload that
	// opens a YAML block is what turns a value into a new field -- AS-02 is
	// exactly that, and the seed carrying securityContext is it.
	f.Add("http://proxy.example.com:8080", "https://proxy.example.com:8443", "localhost,127.0.0.1", "v1", "deadbeef")
	f.Add("", "", "", "v1", "deadbeef")
	f.Add("x\n    securityContext:\n      privileged: true\n    args: |", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("x\n  hostNetwork: true", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("\"", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("\\", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("$(id)", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("\x00", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("\xff\xfe", "https://proxy:8443", "localhost", "v1", "deadbeef")
	f.Add("http://proxy.example.com:8080", "\n  privileged: true", "localhost", "v1", "deadbeef")
	f.Add("http://proxy.example.com:8080", "'", "localhost", "v1", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "a\nb", "v1", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "*", "v1", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "v1\n  x: y", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "\"v1\"", "deadbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "v1", "")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "v1", "dead\nbeef")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "v1", "$(id)")
	f.Add("http://proxy.example.com:8080", "https://proxy:8443", "localhost", "v1", "\xff")

	images := staticPodImagesModel{
		Distribution: "registry.example.com/distribution:v1",
		Auth:         "registry.example.com/auth:v1",
		Mirrorer:     "registry.example.com/mirrorer:v1",
	}

	f.Fuzz(func(t *testing.T, proxyHTTP, proxyHTTPS, noProxy, version, hash string) {
		model := staticPodConfigModel{
			Hash:        hash,
			Version:     version,
			Images:      images,
			HasMirrorer: true,
			Proxy: &staticPodProxyModel{
				HTTP:    proxyHTTP,
				HTTPS:   proxyHTTPS,
				NoProxy: noProxy,
			},
		}

		requireUTF8(t, model, proxyHTTP, proxyHTTPS, noProxy, version, hash)

		reference := model
		reference.Hash = inert(hash)
		reference.Version = inert(version)
		reference.Proxy = &staticPodProxyModel{
			HTTP:    inert(proxyHTTP),
			HTTPS:   inert(proxyHTTPS),
			NoProxy: inert(noProxy),
		}
		assertSameShape(t, model, reference)

		var pod podDoc
		renderAndDecode(t, model, &pod)

		// The manifest must stay the pod the module intends to run.
		if pod.Kind != "Pod" || pod.APIVersion != "v1" {
			t.Fatalf("manifest identity changed: apiVersion=%q kind=%q", pod.APIVersion, pod.Kind)
		}
		if pod.Metadata.Name != "registry-nodeservices" || pod.Metadata.Namespace != "d8-system" {
			t.Fatalf("manifest metadata changed: name=%q namespace=%q",
				pod.Metadata.Name, pod.Metadata.Namespace)
		}
		if pod.Metadata.Annotations["registry.deckhouse.io/config-hash"] != hash {
			t.Fatalf("config-hash annotation corrupted: got %q want %q",
				pod.Metadata.Annotations["registry.deckhouse.io/config-hash"], hash)
		}
		if pod.Metadata.Annotations["registry.deckhouse.io/config-version"] != version {
			t.Fatalf("config-version annotation corrupted: got %q want %q",
				pod.Metadata.Annotations["registry.deckhouse.io/config-version"], version)
		}

		// No container may be added, removed or renamed by the substitution.
		wantContainers := []string{"distribution", "auth", "mirrorer"}
		if len(pod.Spec.Containers) != len(wantContainers) {
			t.Fatalf("container count changed: got %d want %d (%+v)",
				len(pod.Spec.Containers), len(wantContainers), pod.Spec.Containers)
		}
		for i, want := range wantContainers {
			if pod.Spec.Containers[i].Name != want {
				t.Fatalf("container[%d] name changed: got %q want %q",
					i, pod.Spec.Containers[i].Name, want)
			}
		}
		for i, c := range pod.Spec.Containers {
			if c.Image != images.imageFor(c.Name) {
				t.Fatalf("container[%d] %q image changed: got %q", i, c.Name, c.Image)
			}
		}

		// The env block is only rendered when HTTP or HTTPS is set.
		var want []podEnv
		if proxyHTTP != "" || proxyHTTPS != "" {
			if proxyHTTP != "" {
				want = append(want,
					podEnv{Name: "HTTP_PROXY", Value: proxyHTTP},
					podEnv{Name: "http_proxy", Value: proxyHTTP})
			}
			if proxyHTTPS != "" {
				want = append(want,
					podEnv{Name: "HTTPS_PROXY", Value: proxyHTTPS},
					podEnv{Name: "https_proxy", Value: proxyHTTPS})
			}
			if noProxy != "" {
				want = append(want,
					podEnv{Name: "NO_PROXY", Value: noProxy},
					podEnv{Name: "no_proxy", Value: noProxy})
			}
		}

		got := pod.Spec.Containers[0].Env
		if len(got) != len(want) {
			t.Fatalf("env count changed: got %d %+v want %d %+v", len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("env[%d] corrupted: got %+v want %+v", i, got[i], want[i])
			}
		}

		// Only the distribution container carries proxy envs.
		for _, c := range pod.Spec.Containers[1:] {
			if len(c.Env) != 0 {
				t.Fatalf("container %q gained env vars: %+v", c.Name, c.Env)
			}
		}
	})
}

func (m staticPodImagesModel) imageFor(container string) string {
	switch container {
	case "distribution":
		return m.Distribution
	case "auth":
		return m.Auth
	case "mirrorer":
		return m.Mirrorer
	}
	return ""
}

// ---------------------------------------------------------------------------
// TM-08: upstream registry parameters -> distribution config.
// ---------------------------------------------------------------------------

func FuzzDistributionConfigUpstream(f *testing.F) {
	// The seven values that reach the distribution configuration. It is YAML,
	// so the payloads are the ones that end a scalar and start a key; the
	// listen address is also joined with a port, which is where an IPv6
	// literal without brackets goes wrong.
	f.Add("https", "upstream.example.com", "/system/deckhouse", "user", "pass", "10.0.0.1", "secret")
	f.Add("http", "upstream.example.com:5000", "", "", "", "127.0.0.1", "secret")
	f.Add("https\nskip_verify: true", "upstream.example.com", "/x", "user", "pass", "10.0.0.1", "secret")
	f.Add("\"", "upstream.example.com", "/x", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com\n  x: y", "/x", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "\"", "/x", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "$(id)", "/x", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x\n  y: z", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/../../etc", "user", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x", "u\n  y: z", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x", "\\", "pass", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "p\n  y: z", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "'", "10.0.0.1", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "fd00::1", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "[fd00::1]", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "10.0.0.1\n  y: z", "secret")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "10.0.0.1", "")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "10.0.0.1", "s\n  y: z")
	f.Add("https", "upstream.example.com", "/x", "user", "pass", "10.0.0.1", "\xff")

	f.Fuzz(func(t *testing.T, scheme, host, path, user, password, listenAddress, httpSecret string) {
		model := distributionConfigModel{
			ListenAddress: listenAddress,
			HTTPSecret:    httpSecret,
			Upstream: &distributionConfigUpstreamModel{
				Scheme:   scheme,
				Host:     host,
				Path:     path,
				User:     user,
				Password: password,
			},
		}

		requireUTF8(t, model, scheme, host, path, user, password, listenAddress, httpSecret)

		referenceUpstream := *model.Upstream
		referenceUpstream.Scheme = inert(scheme)
		referenceUpstream.Host = inert(host)
		referenceUpstream.Path = inert(path)
		referenceUpstream.User = inert(user)
		referenceUpstream.Password = inert(password)

		reference := model
		reference.ListenAddress = inert(listenAddress)
		reference.HTTPSecret = inert(httpSecret)
		reference.Upstream = &referenceUpstream
		assertSameShape(t, model, reference)

		var doc distributionDoc
		renderAndDecode(t, model, &doc)

		if want := net.JoinHostPort(listenAddress, "5001"); doc.HTTP.Addr != want {
			t.Fatalf("http.addr corrupted: got %q want %q", doc.HTTP.Addr, want)
		}
		if doc.HTTP.Secret != httpSecret {
			t.Fatalf("http.secret corrupted: got %q want %q", doc.HTTP.Secret, httpSecret)
		}
		// TLS must stay mandatory and must not be redirected to another key path.
		if doc.HTTP.TLS["certificate"] != "/pki/distribution.crt" {
			t.Fatalf("http.tls.certificate changed: %+v", doc.HTTP.TLS)
		}
		// realip must not appear: it is only rendered for the Ingress case.
		if doc.HTTP.RealIP != nil {
			t.Fatalf("http.realip appeared unexpectedly: %+v", doc.HTTP.RealIP)
		}

		if doc.Proxy == nil {
			t.Fatalf("proxy section disappeared")
		}
		wantRemote := scheme + "://" + host
		if doc.Proxy.RemoteURL != wantRemote {
			t.Fatalf("proxy.remoteurl corrupted: got %q want %q", doc.Proxy.RemoteURL, wantRemote)
		}
		if doc.Proxy.RemotePathOnly != path {
			t.Fatalf("proxy.remotepathonly corrupted: got %q want %q", doc.Proxy.RemotePathOnly, path)
		}
		if doc.Proxy.LocalPathAlias != "/system/deckhouse" {
			t.Fatalf("proxy.localpathalias changed: got %q", doc.Proxy.LocalPathAlias)
		}
		if user != "" {
			if doc.Proxy.Username != user {
				t.Fatalf("proxy.username corrupted: got %q want %q", doc.Proxy.Username, user)
			}
			if doc.Proxy.Password != password {
				t.Fatalf("proxy.password corrupted: got %q want %q", doc.Proxy.Password, password)
			}
		}
		// The upstream CA is only wired in when the model says so.
		if doc.Proxy.CA != "" {
			t.Fatalf("proxy.ca appeared unexpectedly: %q", doc.Proxy.CA)
		}

		// Auth must keep pointing at the module's own token service.
		wantRealm := "https://" + net.JoinHostPort(listenAddress, "5051") + "/auth"
		if doc.Auth.Token.Realm != wantRealm {
			t.Fatalf("auth.token.realm corrupted: got %q want %q", doc.Auth.Token.Realm, wantRealm)
		}
		if doc.Auth.Token.RootCertBundle != "/pki/token.crt" {
			t.Fatalf("auth.token.rootcertbundle changed: got %q", doc.Auth.Token.RootCertBundle)
		}
		if got := fmt.Sprint(doc.Auth.Token.Proxy["url"]); got != "https://127.0.0.1:5051/auth" {
			t.Fatalf("auth.token.proxy.url changed: got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TM-08: LocalMode.Upstreams -> mirrorer config.
// ---------------------------------------------------------------------------

func FuzzMirrorerConfigUpstreams(f *testing.F) {
	// Two replica addresses, the local address and the two accounts. The
	// addresses are joined with a port and end up in the `remote` list, so the
	// payloads are the ones that add a list entry or break the join.
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("", "", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("fd00::2", "fd00::3", "fd00::1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2\n  - evil", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("\"", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("$(id)", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "\\", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3\n  y: z", "10.0.0.1", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1\n  y: z", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "[fd00::1]", "puller", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "p\n  y: z", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "'", "puller-pass", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "\"", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "p\n  y: z", "pusher", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "", "pusher-pass")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "")
	f.Add("10.0.0.2", "10.0.0.3", "10.0.0.1", "puller", "puller-pass", "pusher", "\xff")

	f.Fuzz(func(t *testing.T, upstream1, upstream2, localAddress, pullerName, pullerPass, pusherName, pusherPass string) {
		upstreams := []string{upstream1, upstream2}

		model := mirrorerConfigModel{
			LocalAddress: localAddress,
			UserPuller:   mirrorerConfigUserModel{Name: pullerName, Password: pullerPass},
			UserPusher:   mirrorerConfigUserModel{Name: pusherName, Password: pusherPass},
			Upstreams:    upstreams,
		}

		requireUTF8(t, model, upstream1, upstream2, localAddress, pullerName, pullerPass, pusherName, pusherPass)

		reference := model
		reference.LocalAddress = inert(localAddress)
		reference.UserPuller = mirrorerConfigUserModel{Name: inert(pullerName), Password: inert(pullerPass)}
		reference.UserPusher = mirrorerConfigUserModel{Name: inert(pusherName), Password: inert(pusherPass)}
		reference.Upstreams = []string{inert(upstream1), inert(upstream2)}
		assertSameShape(t, model, reference)

		var doc mirrorerDoc
		renderAndDecode(t, model, &doc)

		if doc.CA != "/pki/ca.crt" {
			t.Fatalf("ca changed: got %q", doc.CA)
		}
		if want := net.JoinHostPort(localAddress, "5001"); doc.Local != want {
			t.Fatalf("local corrupted: got %q want %q", doc.Local, want)
		}

		// The mirror target list decides which replicas images are pushed to and
		// pulled from: neither its length nor its entries may be influenced beyond
		// the intended one-entry-per-upstream mapping.
		if len(doc.Remote) != len(upstreams) {
			t.Fatalf("remote count changed: got %d %q want %d %q",
				len(doc.Remote), doc.Remote, len(upstreams), upstreams)
		}
		for i, up := range upstreams {
			if want := net.JoinHostPort(up, "5001"); doc.Remote[i] != want {
				t.Fatalf("remote[%d] corrupted: got %q want %q", i, doc.Remote[i], want)
			}
		}

		if len(doc.Users) != 2 {
			t.Fatalf("users count changed: got %d %+v", len(doc.Users), doc.Users)
		}
		if doc.Users["puller"].Name != pullerName || doc.Users["puller"].Password != pullerPass {
			t.Fatalf("puller credentials corrupted: %+v", doc.Users["puller"])
		}
		if doc.Users["pusher"].Name != pusherName || doc.Users["pusher"].Password != pusherPass {
			t.Fatalf("pusher credentials corrupted: %+v", doc.Users["pusher"])
		}
	})
}

// yamlSimpleKeyLimit is the maximum length of a YAML simple key.
const yamlSimpleKeyLimit = 1024

// ---------------------------------------------------------------------------
// Storage users and ACL -> auth (docker_auth) config.
//
// These substitutions already go through `quote`; the harness is a regression
// guard for that escaping and for the "deny by default" ACL shape (TM-15).
// ---------------------------------------------------------------------------

func FuzzAuthConfigUsers(f *testing.F) {
	// Four accounts and their hashes. Each name becomes a YAML mapping key in
	// the auth configuration and an `account` constraint in its ACL, so the
	// payloads are the ones that end a key, add an entry, or make a key too
	// long for the format.
	f.Add("ro", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("", "", "", "", "", "", "", "")
	f.Add("ro\":\n    password: \"x", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("\"", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("\\", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro\n  x: y", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("$(id)", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("\x00", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("\xff\xfe", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "$2a$10$abcdefghijklmnopqrstuv", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "\"", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "h\n  x: y", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "ro", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "rw\n  x: y", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "rw", "'", "puller", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "rw", "rw-hash", "", "puller-hash", "pusher", "pusher-hash")
	f.Add("ro", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "", "pusher-hash")
	f.Add("ro", "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "\xff")
	f.Add(strings.Repeat("a", 300), "ro-hash", "rw", "rw-hash", "puller", "puller-hash", "pusher", "pusher-hash")

	f.Fuzz(func(t *testing.T, roName, roHash, rwName, rwHash, pullerName, pullerHash, pusherName, pusherHash string) {
		// An account name becomes a YAML mapping key, and YAML limits a simple key
		// to 1024 characters: a longer key produces a file no parser accepts, which
		// no amount of escaping can fix. The limit applies to the rendered key, so
		// it is measured on the quoted form -- a control character costs four
		// characters there. nodeservices.User.Validate bounds account names to 255
		// characters of a restricted charset, far below the limit, so the renderer
		// is not contracted to handle longer ones. Every other payload stays in
		// scope: escaping is expected to cover it.
		for _, name := range []string{roName, rwName, pullerName, pusherName} {
			if len(strconv.Quote(name)) >= yamlSimpleKeyLimit {
				t.Skip("account name does not fit a YAML simple key")
			}
		}

		model := authConfigModel{
			RO:           authConfigUserModel{Name: roName, PasswordHash: roHash},
			RW:           &authConfigUserModel{Name: rwName, PasswordHash: rwHash},
			MirrorPuller: &authConfigUserModel{Name: pullerName, PasswordHash: pullerHash},
			MirrorPusher: &authConfigUserModel{Name: pusherName, PasswordHash: pusherHash},
		}

		requireUTF8(t, model, roName, roHash, rwName, rwHash, pullerName, pullerHash, pusherName, pusherHash)

		referenceNames := inertDistinct(roName, rwName, pullerName, pusherName)
		reference := authConfigModel{
			RO:           authConfigUserModel{Name: referenceNames[0], PasswordHash: inert(roHash)},
			RW:           &authConfigUserModel{Name: referenceNames[1], PasswordHash: inert(rwHash)},
			MirrorPuller: &authConfigUserModel{Name: referenceNames[2], PasswordHash: inert(pullerHash)},
			MirrorPusher: &authConfigUserModel{Name: referenceNames[3], PasswordHash: inert(pusherHash)},
		}
		assertSameShape(t, model, reference, "users")

		var doc authDoc
		renderAndDecode(t, model, &doc)

		// Distinct names produce distinct accounts; duplicates collapse. Either way
		// no account beyond the four configured ones may exist.
		names := map[string]string{
			roName:     roHash,
			rwName:     rwHash,
			pullerName: pullerHash,
			pusherName: pusherHash,
		}
		if len(doc.Users) != len(names) {
			t.Fatalf("users count changed: got %d %+v want %d", len(doc.Users), doc.Users, len(names))
		}
		for name := range names {
			user, ok := doc.Users[name]
			if !ok {
				t.Fatalf("account %q missing from rendered users: %+v", name, doc.Users)
			}
			// With duplicate names the last rendered hash wins; only assert that the
			// hash is one of the configured ones and never an injected value.
			if user.Password != roHash && user.Password != rwHash &&
				user.Password != pullerHash && user.Password != pusherHash {
				t.Fatalf("account %q got unexpected password hash %q", name, user.Password)
			}
		}

		// ACL: 1 (ro) + 1 (rw) + 1 (pusher) + 2 (puller) entries, deny by default.
		if len(doc.ACL) != 5 {
			t.Fatalf("acl entry count changed: got %d %+v", len(doc.ACL), doc.ACL)
		}
		for i, entry := range doc.ACL {
			if len(entry.Match) == 0 {
				t.Fatalf("acl[%d] has an empty match, which widens access: %+v", i, entry)
			}
			account, ok := entry.Match["account"]
			if !ok {
				t.Fatalf("acl[%d] lost its account constraint: %+v", i, entry)
			}
			if _, known := names[account]; !known {
				t.Fatalf("acl[%d] references unknown account %q", i, account)
			}
		}
		// The read-only account must never gain write actions. Skipped when its name
		// collides with another account: the template renders one ACL entry per
		// configured account, and the read-write, mirror-pusher and
		// mirror-puller-catalog entries all carry "*" by design, so a shared name
		// legitimately inherits them.
		if roName != rwName && roName != pusherName && roName != pullerName {
			for i, entry := range doc.ACL {
				if entry.Match["account"] != roName {
					continue
				}
				for _, action := range entry.Actions {
					if action != "pull" {
						t.Fatalf("acl[%d] grants %q to the read-only account", i, action)
					}
				}
			}
		}
	})
}
