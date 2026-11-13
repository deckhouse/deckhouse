// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// The in-cluster registry of the registry module. Where the module manages
// the pull path (mode: Managed), the sources of the image set name
// inClusterHost, the deckhouse ModuleSource among them
// (registry.d8-system.svc:5001/system/deckhouse/modules). Only the node agent
// serves that address, on the loopback of each node, while docs-builder runs
// in the pod network. So such a source is read where the agent would fetch it
// (modules/038-registry/images/registry-agent/src/internal/proxy/route.go): a
// repository of the image set from the store, and from the upstream only where
// the store fails, each under its own prefix; a repository beside the image
// set, such as flant/modules, from the upstream at the same path.
//
// The upstream is the one configured in the module's RegistryConfig: the
// agent keeps the previous one while a new one fails its probe and while the
// store fills before air-gap. The store is read under the name of its Service
// with its CA and read-only account, as dhctl in the cluster reads it
// (registry_const.StorageHost in go_lib/registry/const); the name leads to any
// replica, and a follower may not have copied a fresh version yet.
const (
	inClusterHost = "registry.d8-system.svc:5001"
	imageSetPath  = "system/deckhouse"

	storeHost   = "registry.d8-system.svc:4281"
	storeSecret = "registry-storage-client"

	registryNamespace  = "d8-system"
	registryConfigName = "registry"
)

var (
	registryConfigGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "registryconfigs"}
	secretGVR         = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
)

// pullPathTTL keeps what the registry module says for the lookups of the
// next minute: a pass asks for the sources of every module. pullPathTimeout
// bounds one read.
const (
	pullPathTTL     = time.Minute
	pullPathTimeout = 30 * time.Second
)

// pullPath is where the registry module sends a pull of the in-cluster
// address. Where the module does not manage the pull path (managed is false),
// something in the cluster serves the address, and it is read as it is.
type pullPath struct {
	managed bool
	// store is nil without the cache or while it has no replicas; upstream
	// is nil in an air-gapped cluster.
	store, upstream *endpoint
}

// endpoint is a registry of the pull path.
type endpoint struct {
	host, path, scheme, ca string
	// auth is the base64-encoded "login:password", empty for anonymous access.
	auth string
}

// reachable returns the source src is read from: src itself unless it names
// the in-cluster registry of a pull path the registry module manages.
func (s *Sources) reachable(ctx context.Context, src Source) (Source, error) {
	host, repository, _ := strings.Cut(strings.TrimSuffix(src.Repo, "/"), "/")
	if !strings.EqualFold(host, inClusterHost) {
		return src, nil
	}

	path, err := s.pullPath(ctx)
	if err != nil {
		return Source{}, err
	}
	if !path.managed {
		return src, nil
	}

	under, inSet := cutRepository(repository, imageSetPath)
	if !inSet {
		if path.upstream == nil {
			return Source{}, fmt.Errorf("%w: %s is beside the image set %s, which only an upstream serves, and the cluster has none",
				errBadSource, src.Repo, imageSetPath)
		}
		beside := *path.upstream
		beside.path = ""
		return beside.source(src.Name+" (upstream)", repository)
	}

	if path.store == nil && path.upstream == nil {
		return Source{}, fmt.Errorf("the registry module has neither a store nor an upstream to read %s from", src.Repo)
	}
	if path.store == nil {
		return path.upstream.source(src.Name+" (upstream)", under)
	}
	store, err := path.store.source(src.Name+" (store)", under)
	if err != nil {
		return Source{}, err
	}
	if path.upstream == nil {
		return store, nil
	}

	// The upstream is read only where the store fails, as the agent reads
	// them: the store fetches what it does not hold from the upstream itself,
	// so its answer, "not found" and a list of tags too, is final.
	upstream, err := path.upstream.source(src.Name+" (upstream)", under)
	if err != nil {
		return Source{}, err
	}
	store.Fallback = &upstream

	return store, nil
}

// source is the source of repository on the endpoint, under its prefix.
func (e *endpoint) source(name, repository string) (Source, error) {
	repo := e.host
	if p := joinRepository(e.path, repository); p != "" {
		repo += "/" + p
	}

	src := Source{Name: name, Repo: repo, Scheme: e.scheme, CA: e.ca}
	if e.auth == "" {
		return src, nil
	}
	cfg, err := dockerConfig(e.host, e.auth)
	if err != nil {
		return Source{}, err
	}
	src.DockerCfg = cfg

	return src, nil
}

// pullPath returns what the registry module says of its pull path, read at
// most once in pullPathTTL. An error is kept as well, except the one of a
// caller that gave up: it says nothing of the cluster.
func (s *Sources) pullPath(ctx context.Context) (pullPath, error) {
	s.pathMu.Lock()
	defer s.pathMu.Unlock()

	if s.pathAt.IsZero() || time.Since(s.pathAt) >= pullPathTTL {
		path, err := s.readPullPath(ctx)
		if err != nil && ctx.Err() != nil {
			return pullPath{}, err
		}
		s.path, s.pathErr, s.pathAt = path, err, time.Now()
	}

	return s.path, s.pathErr
}

func (s *Sources) readPullPath(ctx context.Context) (pullPath, error) {
	ctx, cancel := context.WithTimeout(ctx, pullPathTimeout)
	defer cancel()

	cfg, err := s.client.Resource(registryConfigGVR).Get(ctx, registryConfigName, metav1.GetOptions{})
	// No RegistryConfig, or no such resource: the module does not manage the
	// pull path.
	if apierrors.IsNotFound(err) {
		return pullPath{}, nil
	}
	if err != nil {
		return pullPath{}, fmt.Errorf("get registry config %s: %w", registryConfigName, err)
	}
	if mode, _, _ := unstructured.NestedString(cfg.Object, "spec", "mode"); mode == "Unmanaged" {
		return pullPath{}, nil
	}

	upstream, err := upstreamFrom(cfg)
	if err != nil {
		return pullPath{}, err
	}
	path := pullPath{managed: true, upstream: upstream}

	if cache, _, _ := unstructured.NestedBool(cfg.Object, "spec", "storage", "cache"); !cache {
		return path, nil
	}
	if path.store, err = s.readStore(ctx); err != nil {
		return pullPath{}, err
	}

	return path, nil
}

// upstreamFrom reads the upstream of the RegistryConfig, nil when there is
// none. Its credentials are a login and a password or pre-encoded, as in a
// docker config; the module takes the pre-encoded ones first.
func upstreamFrom(cfg *unstructured.Unstructured) (*endpoint, error) {
	upstream, _, err := unstructured.NestedMap(cfg.Object, "spec", "primary", "upstream")
	if err != nil {
		return nil, fmt.Errorf("registry config %s: read spec.primary.upstream: %w", registryConfigName, err)
	}
	host, _ := upstream["host"].(string)
	if host == "" {
		return nil, nil
	}

	e := &endpoint{host: host}
	e.path, _ = upstream["path"].(string)
	e.scheme, _ = upstream["scheme"].(string)
	e.ca, _ = upstream["ca"].(string)
	auth, _ := upstream["auth"].(map[string]any)
	e.auth, _ = auth["auth"].(string)
	login, _ := auth["username"].(string)
	password, _ := auth["password"].(string)
	if e.auth == "" && (login != "" || password != "") {
		e.auth = base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
	}

	return e, nil
}

// readStore reads the CA and the read-only account of the store from the
// Secret the registry module keeps for the node agents. nil while the store
// has no replicas: the agents skip it then too.
func (s *Sources) readStore(ctx context.Context) (*endpoint, error) {
	secret, err := s.client.Resource(secretGVR).Namespace(registryNamespace).Get(ctx, storeSecret, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get secret %s/%s: %w", registryNamespace, storeSecret, err)
	}

	data, _, _ := unstructured.NestedStringMap(secret.Object, "data")
	value := make(map[string]string, len(data))
	for key, encoded := range data {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("secret %s/%s: decode %s: %w", registryNamespace, storeSecret, key, err)
		}
		value[key] = string(decoded)
	}
	if value["addresses"] == "" {
		return nil, nil
	}
	if value["ca.crt"] == "" || value["username"] == "" || value["password"] == "" {
		return nil, fmt.Errorf("secret %s/%s has no ca.crt, username or password", registryNamespace, storeSecret)
	}

	return &endpoint{
		host:   storeHost,
		path:   imageSetPath,
		scheme: "HTTPS",
		ca:     value["ca.crt"],
		auth:   base64.StdEncoding.EncodeToString([]byte(value["username"] + ":" + value["password"])),
	}, nil
}

// cutRepository returns repository relative to prefix and whether it is
// prefix or under it: system/deckhouse-extra is beside system/deckhouse.
func cutRepository(repository, prefix string) (string, bool) {
	if repository == prefix {
		return "", true
	}
	return strings.CutPrefix(repository, prefix+"/")
}

// joinRepository puts repository under prefix. The path of an upstream is as
// an administrator typed it: "/deckhouse/ee", "deckhouse/ee/" or empty.
func joinRepository(prefix, repository string) string {
	prefix, repository = strings.Trim(prefix, "/"), strings.Trim(repository, "/")
	switch {
	case prefix == "":
		return repository
	case repository == "":
		return prefix
	}
	return prefix + "/" + repository
}
