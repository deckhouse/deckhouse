// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"context"

	"github.com/deckhouse/lib-connection/pkg/kube"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/kubeerrors"
)

// WithKubeAuthMode stamps ctx with how this invocation reaches the apiserver, which is what
// decides whether a 401/403 can be retried — see kubeerrors.AuthMode. Call it once, on the
// context the operation runs under; every retry loop below inherits it.
func WithKubeAuthMode(ctx context.Context, kubeOpts *options.KubeOptions) context.Context {
	return kubeerrors.WithAuthMode(ctx, KubeAuthMode(kubeOpts))
}

// KubeAuthMode maps the kube flags onto the auth mode they produce. The predicate is
// kube.Config.OverSSH(), the same one lib-connection uses to decide whether to tunnel to a
// kubectl proxy on a master — so the two cannot disagree about which mode is in play.
func KubeAuthMode(kubeOpts *options.KubeOptions) kubeerrors.AuthMode {
	cfg := &kube.Config{}
	if kubeOpts != nil {
		cfg.KubeConfig = kubeOpts.Config
		cfg.KubeConfigContext = kubeOpts.ConfigContext
		cfg.KubeConfigInCluster = kubeOpts.InCluster
	}

	if cfg.OverSSH() {
		return kubeerrors.AuthModeKubeProxy
	}

	return kubeerrors.AuthModeOwnCredentials
}
