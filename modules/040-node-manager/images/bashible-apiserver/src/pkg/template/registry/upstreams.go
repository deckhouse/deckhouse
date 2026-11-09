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

package registry

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
)

// RegistryUpstream objects reach a containerd v1 node through `.registry.inlineUpstreams`,
// which step 032 renders as inline mirrors in config.toml. Only while the registry module
// configures no node, that is while its node configuration (Secret registry-bashible-config)
// is absent: the module is disabled or in Unmanaged mode. Step 032 then sets no config_path,
// so containerd v1 reads its mirrors from config.toml and never from registry.d, where
// nodelet writes the same objects for every other node (NodeConfig spec.registry.upstreams).
// While the module configures the nodes, containerd v1 reads registry.d like every other
// runtime, and nodelet's routes (or the registry agent) serve it there. So the steps that
// write registry.d never see these, and nodelet's directories are never written twice.
//
// The arbitration is node-controller's, the one arbitrator while no agent runs: an object
// is rendered here once that controller has accepted it, for the generation it has now.
// One whose spec moved since waits for its next verdict rather than being rendered on the
// strength of the last one.
//
// Everything here ends up in a shell script that runs as root, and the objects are written
// by users. So every value is checked against what it must be rather than what it must not
// contain, and an object that fails is left out on its own instead of failing the whole
// registry context.

var (
	// repositoryPathRegexp is the repository prefix: OCI repository name characters only.
	repositoryPathRegexp = regexp.MustCompile(`^[a-z0-9]+([._/-][a-z0-9]+)*$`)
	// base64Regexp is a pre-encoded `user:password` pair.
	base64Regexp = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
)

// registryUpstreamGVK is read as unstructured: the typed API lives in go_lib/registry/apis,
// a module this server deliberately does not depend on (see its go.mod).
var registryUpstreamGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "RegistryUpstream"}

// registryUpstream is the part of a RegistryUpstream rendered on the nodes
// (modules/038-registry/crds/registryupstream.yaml), and node-controller's verdict on it.
type registryUpstream struct {
	metav1.ObjectMeta `json:"metadata"`
	Spec              struct {
		Match    string           `json:"match"`
		Upstream upstreamEndpoint `json:"upstream"`
	} `json:"spec"`
	Status struct {
		Accepted           bool  `json:"accepted,omitempty"`
		ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	} `json:"status"`
}

type upstreamEndpoint struct {
	Scheme string `json:"scheme,omitempty"`
	Host   string `json:"host"`
	Path   string `json:"path,omitempty"`
	CA     string `json:"ca,omitempty"`
	Auth   *struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Auth     string `json:"auth,omitempty"`
	} `json:"auth,omitempty"`
	Mirrors []upstreamEndpoint `json:"mirrors,omitempty"`
}

func newUpstreamObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(registryUpstreamGVK)
	return obj
}

// readUpstreams converts the listed objects; one that does not convert is left out and logged.
func readUpstreams(items []unstructured.Unstructured) []registryUpstream {
	upstreams := make([]registryUpstream, 0, len(items))
	for i := range items {
		var upstream registryUpstream
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(items[i].Object, &upstream); err != nil {
			klog.Warningf("RegistryUpstream %q is not rendered: %v", items[i].GetName(), err)
			continue
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams
}

// upstreamVerdicts maps an object name to why it was left out.
type upstreamVerdicts map[string]string

// addUpstreams puts the upstreams node-controller accepted into rd.InlineUpstreams.
//
// Inline mirrors have no repository rewrite, so an upstream with a path, on its registry
// or on a mirror, is left out rather than pulled from the wrong repository. A match among
// rd.Hosts is left out too: node-controller never accepts the registry the cluster pulls
// from, and this is the last place to make sure. So is one naming a registry configured
// already, by itself or by an object before it. node-controller applies the same rules,
// in the same order, to say on the object's Accepted condition what is left out here
// (nodeconfig/upstreams.go, containerdV1Notes): a change to one belongs in the other.
func (rd *RegistryData) addUpstreams(upstreams []registryUpstream) upstreamVerdicts {
	// Oldest first, then by name, as the arbitration orders them: the same object keeps a
	// registry here on every pass.
	sort.SliceStable(upstreams, func(i, j int) bool {
		left, right := &upstreams[i], &upstreams[j]
		if !left.CreationTimestamp.Equal(&right.CreationTimestamp) {
			return left.CreationTimestamp.Before(&right.CreationTimestamp)
		}
		return left.Name < right.Name
	})

	rejected := upstreamVerdicts{}
	inline := map[string]bashible.ContextHosts{}
	// config.toml keys the credentials and the CA by the registry host, not by the host a
	// pull names, so one registry has one table there: a second makes the file unparsable.
	configured := map[string]bool{}
	for _, values := range rd.Hosts {
		for _, mirror := range values.Mirrors {
			configured[normalizeHost(mirror.Host)] = true
		}
	}
	for i := range upstreams {
		upstream := &upstreams[i]
		if !upstream.Status.Accepted || upstream.Status.ObservedGeneration != upstream.Generation {
			continue
		}
		match := normalizeHost(upstream.Spec.Match)
		if err := helpers.HostWithOptionalPort(match); err != nil {
			rejected[upstream.Name] = fmt.Sprintf("spec.match %q: %v", upstream.Spec.Match, err)
			continue
		}
		if _, taken := rd.Hosts[match]; taken || match == normalizeHost(registry_const.Host) {
			rejected[upstream.Name] = fmt.Sprintf("spec.match %q is a registry the cluster pulls from", upstream.Spec.Match)
			continue
		}
		if _, taken := inline[match]; taken {
			rejected[upstream.Name] = fmt.Sprintf("spec.match %q is rendered for another RegistryUpstream", upstream.Spec.Match)
			continue
		}
		hosts, err := upstreamHosts(upstream.Spec.Upstream)
		if err != nil {
			rejected[upstream.Name] = err.Error()
			continue
		}
		if slices.ContainsFunc(hosts.Mirrors, func(m bashible.ContextMirrorHost) bool { return len(m.Rewrites) > 0 }) {
			rejected[upstream.Name] = "a path cannot be applied by containerd v1, which reads its mirrors from config.toml"
			continue
		}
		// By the object's own addresses too: one host over both schemes is two mirrors here
		// and one table there.
		own := map[string]bool{}
		if i := slices.IndexFunc(hosts.Mirrors, func(m bashible.ContextMirrorHost) bool {
			repeated := configured[m.Host] || own[m.Host]
			own[m.Host] = true
			return repeated
		}); i >= 0 {
			rejected[upstream.Name] = fmt.Sprintf("registry %q is configured already, and containerd v1 holds one configuration per registry", hosts.Mirrors[i].Host)
			continue
		}
		for _, mirror := range hosts.Mirrors {
			configured[mirror.Host] = true
		}
		inline[match] = hosts
	}
	if len(inline) > 0 {
		rd.InlineUpstreams = inline
	}
	return rejected
}

// upstreamHosts turns the upstream and its mirrors into the mirrors of one host entry.
func upstreamHosts(upstream upstreamEndpoint) (bashible.ContextHosts, error) {
	endpoints := append([]upstreamEndpoint{upstream}, upstream.Mirrors...)
	hosts := bashible.ContextHosts{}
	seen := map[string]bool{}
	for i, endpoint := range endpoints {
		mirror, err := upstreamMirror(endpoint)
		if err != nil {
			if i == 0 {
				return hosts, fmt.Errorf("spec.upstream: %w", err)
			}
			return hosts, fmt.Errorf("spec.upstream.mirrors[%d]: %w", i-1, err)
		}
		// The step names the CA file after the scheme and the host, so two entries
		// for one address would overwrite each other's.
		key := mirror.UniqueKey()
		if seen[key] {
			continue
		}
		seen[key] = true
		hosts.Mirrors = append(hosts.Mirrors, mirror)
	}
	return hosts, nil
}

func upstreamMirror(endpoint upstreamEndpoint) (bashible.ContextMirrorHost, error) {
	mirror := bashible.ContextMirrorHost{Scheme: "https", Host: normalizeHost(endpoint.Host)}
	if strings.EqualFold(endpoint.Scheme, "http") {
		mirror.Scheme = "http"
	}
	if err := helpers.HostWithOptionalPort(mirror.Host); err != nil {
		return mirror, fmt.Errorf("host %q: %w", endpoint.Host, err)
	}

	if path := strings.Trim(endpoint.Path, "/"); path != "" {
		if !repositoryPathRegexp.MatchString(path) {
			return mirror, fmt.Errorf("path %q is not a repository path", endpoint.Path)
		}
		// The containerd rewrite patch replaces the regex match within the repository
		// name; an anchored empty match prefixes it.
		mirror.Rewrites = []bashible.ContextRewrite{{From: "^", To: path + "/"}}
	}

	if mirror.Scheme == "https" && strings.TrimSpace(endpoint.CA) != "" {
		ca, err := canonicalCertificates(endpoint.CA)
		if err != nil {
			return mirror, fmt.Errorf("ca: %w", err)
		}
		mirror.CA = ca
	}

	if auth := endpoint.Auth; auth != nil {
		// The pair is base64-encoded by the template; a pre-encoded one is rendered as is.
		if auth.Auth != "" && !base64Regexp.MatchString(auth.Auth) {
			return mirror, errors.New("auth.auth is not base64")
		}
		mirror.Auth = bashible.ContextAuth{Username: auth.Username, Password: auth.Password, Auth: auth.Auth}
		if auth.Auth != "" {
			// The CRD gives the pre-encoded pair precedence; the template the opposite.
			mirror.Auth.Username, mirror.Auth.Password = "", ""
		}
	}
	return mirror, nil
}

// canonicalCertificates re-encodes a PEM bundle of certificates. The CA lands in a
// heredoc, so it is never copied as written: only parsed certificates come out.
func canonicalCertificates(bundle string) (string, error) {
	var out bytes.Buffer
	rest := []byte(bundle)
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return "", fmt.Errorf("a PEM block of type %q, want CERTIFICATE", block.Type)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return "", fmt.Errorf("parse certificate: %w", err)
		}
		if err := pem.Encode(&out, &pem.Block{Type: "CERTIFICATE", Bytes: block.Bytes}); err != nil {
			return "", fmt.Errorf("encode certificate: %w", err)
		}
		rest = remaining
	}
	if len(bytes.TrimSpace(rest)) != 0 || out.Len() == 0 {
		return "", errors.New("not a PEM bundle of certificates")
	}
	return strings.TrimSpace(out.String()), nil
}

// normalizeHost compares hosts the way DNS does, so a capitalised match cannot slip past.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
