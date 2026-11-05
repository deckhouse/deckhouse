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

package scope

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// The ConfigMap kube-apiserver publishes its front-proxy settings in.
const (
	AuthenticationConfigMapNamespace = "kube-system"
	AuthenticationConfigMapName      = "extension-apiserver-authentication"

	keyRequestHeaderCA              = "requestheader-client-ca-file"
	keyRequestHeaderAllowedNames    = "requestheader-allowed-names"
	keyRequestHeaderUsernameHeaders = "requestheader-username-headers"
)

// ConfigMapTrustSource is a TrustSource that follows
// kube-system/extension-apiserver-authentication with an informer, so a
// rotated requestheader CA is picked up without a restart. A deleted or
// malformed ConfigMap trusts nothing until a valid one appears.
//
// The ServiceAccount needs get/list/watch on that ConfigMap, which the
// Role kube-system/extension-apiserver-authentication-reader grants.
type ConfigMapTrustSource struct {
	informer cache.SharedIndexInformer
	factory  informers.SharedInformerFactory
	current  atomic.Pointer[TrustConfig]
}

// NewConfigMapTrustSource creates the source; call Run to start it.
func NewConfigMapTrustSource(client kubernetes.Interface, resync time.Duration) *ConfigMapTrustSource {
	factory := informers.NewSharedInformerFactoryWithOptions(client, resync,
		informers.WithNamespace(AuthenticationConfigMapNamespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.FieldSelector = fields.OneTermEqualSelector("metadata.name", AuthenticationConfigMapName).String()
		}),
	)
	s := &ConfigMapTrustSource{
		factory:  factory,
		informer: factory.Core().V1().ConfigMaps().Informer(),
	}
	_, _ = s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { s.update(obj) },
		UpdateFunc: func(_, obj any) { s.update(obj) },
		DeleteFunc: func(any) {
			klog.InfoS("scope: authentication ConfigMap deleted, trusting nothing")
			s.current.Store(nil)
		},
	})
	return s
}

// Run starts the informer and blocks until ctx is done.
func (s *ConfigMapTrustSource) Run(ctx context.Context) {
	s.factory.Start(ctx.Done())
	<-ctx.Done()
	s.factory.Shutdown()
}

// HasSynced reports whether the informer has seen the ConfigMap state once.
func (s *ConfigMapTrustSource) HasSynced() bool { return s.informer.HasSynced() }

func (s *ConfigMapTrustSource) TrustConfig() (TrustConfig, bool) {
	cfg := s.current.Load()
	if cfg == nil {
		return TrustConfig{}, false
	}
	return *cfg, true
}

func (s *ConfigMapTrustSource) update(obj any) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return
	}
	cfg, err := ParseAuthenticationConfigMap(cm)
	if err != nil {
		klog.ErrorS(err, "scope: invalid authentication ConfigMap, trusting nothing")
		s.current.Store(nil)
		return
	}
	klog.V(2).InfoS("scope: loaded the authentication ConfigMap", "resourceVersion", cm.ResourceVersion, "allowedNames", cfg.AllowedNames, "usernameHeaders", cfg.UsernameHeaders)
	s.current.Store(&cfg)
}

// ParseAuthenticationConfigMap reads the front-proxy settings from the
// ConfigMap kube-apiserver publishes. The list values are JSON arrays.
func ParseAuthenticationConfigMap(cm *corev1.ConfigMap) (TrustConfig, error) {
	pem := cm.Data[keyRequestHeaderCA]
	if pem == "" {
		return TrustConfig{}, fmt.Errorf("%s is empty", keyRequestHeaderCA)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(pem)) {
		return TrustConfig{}, fmt.Errorf("%s has no certificates", keyRequestHeaderCA)
	}
	var cfg TrustConfig
	cfg.Roots = pool
	for key, dst := range map[string]*[]string{
		keyRequestHeaderAllowedNames:    &cfg.AllowedNames,
		keyRequestHeaderUsernameHeaders: &cfg.UsernameHeaders,
	} {
		raw, ok := cm.Data[key]
		if !ok || raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(raw), dst); err != nil {
			return TrustConfig{}, fmt.Errorf("%s: %w", key, err)
		}
	}
	return cfg, nil
}
