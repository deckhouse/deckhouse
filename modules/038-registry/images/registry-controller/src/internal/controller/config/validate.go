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

package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Validate checks a RegistryConfig for the mistakes the CRD schema cannot catch.
//
// The schema already enforces shape, enums and the two-axis invariants through
// CEL. What is left here is everything that needs parsing (a host, a quantity, a
// base64 blob) or cross-field comparison over a list, plus the invariants
// repeated as defence in depth: the CR is a multi-writer contract, so it can be
// written by an administrator by hand during an incident, and a relaxed schema
// must not silently become a broken pull path.
func Validate(cfg *registryv1alpha1.RegistryConfig) field.ErrorList {
	var errs field.ErrorList

	if cfg.Name != registryv1alpha1.SingletonName {
		// Reported rather than ignored: a stray object that looks configured but
		// does nothing is worse than one that says why.
		errs = append(errs, field.Invalid(
			field.NewPath("metadata", "name"), cfg.Name,
			fmt.Sprintf("only the singleton object %q is used", registryv1alpha1.SingletonName),
		))
	}

	spec := field.NewPath("spec")
	errs = append(errs, validateAxes(spec, &cfg.Spec)...)

	if cfg.Spec.Primary.Upstream != nil {
		errs = append(errs, validateUpstream(spec.Child("primary", "upstream"), cfg.Spec.Primary.Upstream)...)
	}
	errs = append(errs, validateStorage(spec.Child("storage"), &cfg.Spec.Storage)...)

	return errs
}

// validateAxes repeats the two-axis invariants the CRD enforces with CEL.
func validateAxes(path *field.Path, spec *registryv1alpha1.RegistryConfigSpec) field.ErrorList {
	if spec.Mode == registryv1alpha1.ModeUnmanaged {
		// Unmanaged owns nothing, so it needs nothing.
		return nil
	}

	var errs field.ErrorList

	hasUpstream := spec.Primary.Upstream != nil
	if !hasUpstream && !spec.Storage.Cache {
		errs = append(errs, field.Invalid(path, "", "a Managed registry needs a source of images: "+
			"set primary.upstream, or enable storage.cache for an air-gapped cluster"))
	}

	if !hasUpstream && spec.Storage.Source == nil {
		errs = append(errs, field.Required(path.Child("storage", "source"),
			"required without primary.upstream: in air-gap there is no upstream to fall back on, "+
				"so cache completeness must be decidable"))
	}

	return errs
}

func validateUpstream(path *field.Path, upstream *registryv1alpha1.Upstream) field.ErrorList {
	errs := validateEndpoint(path, &upstream.Endpoint)

	seen := map[string]int{upstream.UniqueKey(): -1}

	for i := range upstream.Mirrors {
		mirror := &upstream.Mirrors[i]
		mirrorPath := path.Child("mirrors").Index(i)

		errs = append(errs, validateEndpoint(mirrorPath, mirror)...)

		// Mirrors serve the same content, so a duplicate is dead weight at best
		// and, once the syncer starts failing over between them, a source of
		// confusing behaviour.
		key := mirror.UniqueKey()
		if prev, ok := seen[key]; ok {
			if prev == -1 {
				errs = append(errs, field.Duplicate(mirrorPath, "same endpoint as the primary upstream"))
			} else {
				errs = append(errs, field.Duplicate(mirrorPath, fmt.Sprintf("same endpoint as mirrors[%d]", prev)))
			}
			continue
		}
		seen[key] = i
	}

	return errs
}

func validateEndpoint(path *field.Path, endpoint *registryv1alpha1.Endpoint) field.ErrorList {
	var errs field.ErrorList

	errs = append(errs, validateHost(path.Child("host"), endpoint.Host)...)

	if strings.Contains(endpoint.Path, "://") {
		errs = append(errs, field.Invalid(path.Child("path"), endpoint.Path,
			"must be a repository prefix, not a URL"))
	}
	if strings.ContainsAny(endpoint.Path, " \t\n") {
		errs = append(errs, field.Invalid(path.Child("path"), endpoint.Path, "must not contain whitespace"))
	}

	switch endpoint.Scheme {
	case "", registryv1alpha1.SchemeHTTP, registryv1alpha1.SchemeHTTPS:
	default:
		errs = append(errs, field.NotSupported(path.Child("scheme"), endpoint.Scheme,
			[]string{string(registryv1alpha1.SchemeHTTP), string(registryv1alpha1.SchemeHTTPS)}))
	}

	if endpoint.Scheme == registryv1alpha1.SchemeHTTP && endpoint.CA != "" {
		// Not fatal, but it means someone expects TLS that will not happen.
		errs = append(errs, field.Invalid(path.Child("ca"), "<omitted>",
			"a certificate authority is meaningless with scheme HTTP"))
	}

	errs = append(errs, validateAuth(path.Child("auth"), endpoint.Auth)...)

	return errs
}

// validateHost accepts a registry host with an optional port, and rejects the
// two mistakes that actually happen: pasting a URL, and pasting a full image
// reference.
func validateHost(path *field.Path, host string) field.ErrorList {
	if host == "" {
		return field.ErrorList{field.Required(path, "")}
	}
	if strings.Contains(host, "://") {
		return field.ErrorList{field.Invalid(path, host, "must be a host, optionally with a port, not a URL")}
	}
	if strings.Contains(host, "/") {
		return field.ErrorList{field.Invalid(path, host, "must not contain a path; use the 'path' field")}
	}

	hostname := host
	if strings.LastIndex(host, ":") > strings.LastIndex(host, "]") {
		var port string
		var err error
		hostname, port, err = net.SplitHostPort(host)
		if err != nil {
			return field.ErrorList{field.Invalid(path, host, "is not a valid host:port pair")}
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return field.ErrorList{field.Invalid(path, host, "port must be a number between 1 and 65535")}
		}
	}

	if hostname == "" {
		return field.ErrorList{field.Invalid(path, host, "host part must not be empty")}
	}
	if strings.HasPrefix(hostname, "[") {
		// An IPv6 literal, already validated by SplitHostPort.
		return nil
	}
	if net.ParseIP(hostname) != nil {
		return nil
	}
	if strings.ContainsAny(hostname, " \t\n") {
		return field.ErrorList{field.Invalid(path, host, "must not contain whitespace")}
	}
	for _, label := range strings.Split(hostname, ".") {
		if label == "" {
			return field.ErrorList{field.Invalid(path, host, "must not contain an empty domain label")}
		}
	}

	return nil
}

func validateAuth(path *field.Path, auth *registryv1alpha1.Auth) field.ErrorList {
	if auth.IsEmpty() {
		return nil
	}

	var errs field.ErrorList

	if auth.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
		if err != nil {
			errs = append(errs, field.Invalid(path.Child("auth"), "<omitted>",
				"must be base64-encoded \"username:password\""))
		} else if !strings.Contains(string(decoded), ":") {
			errs = append(errs, field.Invalid(path.Child("auth"), "<omitted>",
				"decodes to a value without a \":\" separator; expected \"username:password\""))
		}
	}

	// A half-filled pair is almost always a mistake, and it fails at pull time
	// with an opaque 401 rather than here.
	if auth.Auth == "" {
		if auth.Username == "" && auth.Password != "" {
			errs = append(errs, field.Required(path.Child("username"), "required when a password is set"))
		}
		if auth.Username != "" && auth.Password == "" {
			errs = append(errs, field.Required(path.Child("password"), "required when a username is set"))
		}
	}

	return errs
}

func validateStorage(path *field.Path, storage *registryv1alpha1.StorageConfig) field.ErrorList {
	var errs field.ErrorList

	if storage.Size != "" {
		quantity, err := resource.ParseQuantity(storage.Size)
		switch {
		case err != nil:
			errs = append(errs, field.Invalid(path.Child("size"), storage.Size, "is not a valid quantity"))
		case quantity.Sign() <= 0:
			errs = append(errs, field.Invalid(path.Child("size"), storage.Size, "must be greater than zero"))
		}
	}

	if storage.Source != nil && storage.Source.ExpectedDigests < 0 {
		errs = append(errs, field.Invalid(path.Child("source", "expectedDigests"),
			storage.Source.ExpectedDigests, "must not be negative"))
	}

	return errs
}
