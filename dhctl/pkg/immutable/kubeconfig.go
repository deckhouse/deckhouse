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

package immutable

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// ErrKubeconfigOutRequired stops an immutable bootstrap driven by dhctl-server:
// the admin kubeconfig is the only way into the cluster, the bootstrap response
// does not carry it, and the server never sets --kubeconfig-out to keep it.
var ErrKubeconfigOutRequired = errors.New(
	"Commander cannot bootstrap an immutable master: the admin kubeconfig is collected from the node once and " +
		"is the only way into such a cluster, and nothing in the bootstrap response carries it back. " +
		"Bootstrap it from the dhctl CLI instead, with --kubeconfig-out naming where to keep the kubeconfig",
)

// CheckKubeconfigOutSurvivesCleanup rejects a --kubeconfig-out the tmp cleaner
// would sweep at exit (under tmpDir, without a protected suffix): on an
// immutable master that file is the only way in.
func CheckKubeconfigOutSurvivesCleanup(_ context.Context, kubeconfigOut, tmpDir string) error {
	if kubeconfigOut == "" || tmpDir == "" {
		return nil
	}

	// Symlinks resolved, not just made absolute: on macOS /tmp -> /private/tmp
	// would make a path under the cleaner's directory compare as outside it.
	// A path that does not exist yet keeps its lexical form.
	out := resolvePath(kubeconfigOut)
	tmp := resolvePath(tmpDir)

	if !strings.HasPrefix(out, tmp+string(filepath.Separator)) {
		return nil
	}
	if strings.HasSuffix(out, cache.AdminKubeconfigName) {
		return nil
	}

	return fmt.Errorf(
		"--kubeconfig-out %s is inside the dhctl temporary directory %s, which dhctl empties when it exits, "+
			"and on a cluster whose first master is immutable that file is the only way in: "+
			"name a path outside that directory, or one ending in %q",
		kubeconfigOut, tmpDir, cache.AdminKubeconfigName,
	)
}

// resolvePath makes a path absolute and follows symlinks on its directory,
// keeping the last element lexically: the checked file does not exist yet, so
// resolving the whole path would do nothing at all.
func resolvePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	dir, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return absolute
	}
	return filepath.Join(dir, filepath.Base(absolute))
}

// RetargetKubeconfig points the collected admin kubeconfig at the address dhctl
// reaches the API server on (e.g. a bastion's local forward) and at the name its
// certificate is issued for. The operator's own copy keeps the node's address.
func RetargetKubeconfig(_ context.Context, content []byte, server, serverName string) ([]byte, error) {
	if server == "" {
		return nil, errors.New("retarget the admin kubeconfig: server URL is empty")
	}
	if serverName == "" {
		return nil, errors.New("retarget the admin kubeconfig: server name is empty")
	}

	kubeconfig, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("parse the collected admin kubeconfig: %w", err)
	}
	if len(kubeconfig.Clusters) == 0 {
		return nil, errors.New("the collected admin kubeconfig names no cluster")
	}

	for _, cluster := range kubeconfig.Clusters {
		cluster.Server = server
		// The node's apiserver certificate covers its own addresses, 127.0.0.1 and
		// the operator's certSANs — all of them chosen before the VM existed, so a
		// NAT address dhctl dials is in none of them. The node name always is.
		cluster.TLSServerName = serverName
	}

	out, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the collected admin kubeconfig: %w", err)
	}

	return out, nil
}

// SaveCollectedKubeconfig records where the admin kubeconfig the node served now
// lives.
func SaveCollectedKubeconfig(ctx context.Context, cache state.Cache, adminKubeconfigPath string) error {
	if err := cache.Save(ctx, collectedKubeconfigCacheKey, []byte(adminKubeconfigPath)); err != nil {
		return fmt.Errorf("record the collected admin kubeconfig in the state cache: %w", err)
	}
	return nil
}

// LoadCollectedKubeconfig returns the path SaveCollectedKubeconfig stored, or ""
// when no attempt has collected the credentials yet.
func LoadCollectedKubeconfig(ctx context.Context, cache state.Cache) (string, error) {
	inCache, err := cache.InCache(ctx, collectedKubeconfigCacheKey)
	if err != nil {
		return "", fmt.Errorf("look up %s in the state cache: %w", collectedKubeconfigCacheKey, err)
	}
	if !inCache {
		return "", nil
	}

	path, err := cache.Load(ctx, collectedKubeconfigCacheKey)
	if err != nil {
		return "", fmt.Errorf("load %s from the state cache: %w", collectedKubeconfigCacheKey, err)
	}
	return string(path), nil
}

// CompleteAdminKubeconfig pairs the document the node handed back with the
// private key it was never given: the node returns only a public certificate,
// and the key stayed in the state cache, so this is where the halves meet.
func CompleteAdminKubeconfig(ctx context.Context, cache state.Cache, collected []byte) ([]byte, error) {
	material, err := LoadHandoffMaterial(ctx, cache)
	if err != nil {
		return nil, err
	}
	if material == nil {
		return nil, errors.New("the installer's client key is missing from the state cache: rerun the bootstrap so the BaseInfra phase regenerates the master payload")
	}
	return withClientKey(collected, material.ClientKeyPEM)
}

func withClientKey(content []byte, clientKeyPEM string) ([]byte, error) {
	if clientKeyPEM == "" {
		return nil, errors.New("complete the admin kubeconfig: the installer's client key is empty")
	}

	kubeconfig, err := clientcmd.Load(content)
	if err != nil {
		return nil, fmt.Errorf("parse the collected admin kubeconfig: %w", err)
	}
	if len(kubeconfig.AuthInfos) == 0 {
		return nil, errors.New("the collected admin kubeconfig names no user")
	}

	for _, authInfo := range kubeconfig.AuthInfos {
		if len(authInfo.ClientCertificateData) == 0 {
			return nil, errors.New("the collected admin kubeconfig carries no client certificate")
		}
		// A node that returned a key would mean the channel carried a secret
		// after all; refuse rather than quietly use it.
		if len(authInfo.ClientKeyData) != 0 {
			return nil, errors.New("the node returned a private key, which this channel must never carry")
		}
		authInfo.ClientKeyData = []byte(clientKeyPEM)
	}

	out, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("serialize the admin kubeconfig: %w", err)
	}
	return out, nil
}
