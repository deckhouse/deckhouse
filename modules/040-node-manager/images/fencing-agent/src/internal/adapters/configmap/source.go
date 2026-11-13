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

// Package configmap gives the agent configuration: the fencing-agent-config
// ConfigMap, the node group of the own Node and the FencingSLAProfiles.
package configmap

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/config"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/profile"
)

const (
	// retryInterval paces the GET of a profile that is not ready.
	retryInterval = 2 * time.Second
)

// NodeSource gives the state of the own Node.
type NodeSource interface {
	Snapshot() (domain.NodeSignals, bool)
	// Changed receives a value after the state of the Node changes.
	Changed() <-chan struct{}
}

// Source builds the Settings from the ConfigMap, the node group and the
// profiles. It sends a Settings only when it changes. A value that is not valid
// is not applied: the previous one stays in force.
type Source struct {
	factory  informers.SharedInformerFactory
	informer cache.SharedIndexInformer
	self     NodeSource
	profiles profile.Getter
	metrics  domain.Metrics
	logger   *log.Logger

	retryInterval time.Duration

	kick    chan struct{}
	updates chan config.Settings

	// The fields below belong to the goroutine of Run.
	current config.Settings
	applied bool
	handled input
	missing bool
}

// input is what the last decision was made from.
type input struct {
	config string
	group  string
}

// NewSource watches the ConfigMap by name. It does not watch any other object.
func NewSource(client kubernetes.Interface, self NodeSource, profiles profile.Getter, metrics domain.Metrics, logger *log.Logger) (*Source, error) {
	selector := fields.OneTermEqualSelector("metadata.name", config.ConfigMapName).String()

	factory := informers.NewSharedInformerFactoryWithOptions(client, 0,
		informers.WithNamespace(config.ConfigMapNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = selector
		}),
	)

	s := &Source{
		factory:       factory,
		informer:      factory.Core().V1().ConfigMaps().Informer(),
		self:          self,
		profiles:      profiles,
		metrics:       metrics,
		logger:        logger,
		retryInterval: retryInterval,
		kick:          make(chan struct{}, 1),
		updates:       make(chan config.Settings),
	}

	if _, err := s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { s.notify() },
		UpdateFunc: func(_, _ any) { s.notify() },
		DeleteFunc: func(any) { s.notify() },
	}); err != nil {
		return nil, fmt.Errorf("add configmap event handler: %w", err)
	}

	return s, nil
}

// Updates delivers each new Settings. The first one comes when the ConfigMap,
// the own Node and the profiles are ready.
func (s *Source) Updates() <-chan config.Settings {
	return s.updates
}

func (s *Source) notify() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Run blocks until ctx ends.
func (s *Source) Run(ctx context.Context) error {
	s.factory.Start(ctx.Done())
	defer s.factory.Shutdown()

	if !cache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced) {
		return nil
	}

	changed := s.self.Changed()

	var retry <-chan time.Time

	for {
		retry = nil
		if delay := s.reconcile(ctx); delay > 0 {
			retry = time.After(delay)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-s.kick:
		case <-changed:
		case <-retry:
		}
	}
}

// reconcile brings the Settings up to date. It returns the delay of the next
// call, or zero when the next change of the ConfigMap or the Node calls it.
func (s *Source) reconcile(ctx context.Context) time.Duration {
	signals, ok := s.self.Snapshot()
	if !ok {
		return 0
	}

	obj, exists, err := s.informer.GetStore().GetByKey(config.ConfigMapNamespace + "/" + config.ConfigMapName)
	if err != nil || !exists {
		if !s.missing {
			s.logger.Warn("agent configmap not found", "name", config.ConfigMapName, "error", err)
			s.missing = true
		}

		return 0
	}

	s.missing = false

	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return 0
	}

	in := input{config: cm.Data[config.ConfigMapKey], group: signals.NodeGroup}
	if in == s.handled {
		return 0
	}

	done, invalid, settings := s.build(ctx, in)
	s.report(invalid)

	if !done {
		return s.retryInterval
	}

	s.handled = in

	if settings != nil && (!s.applied || !reflect.DeepEqual(*settings, s.current)) {
		select {
		case s.updates <- *settings:
			s.current, s.applied = *settings, true
		case <-ctx.Done():
		}
	}

	return 0
}

// build makes the Settings from in. It returns nil Settings when nothing can
// be applied, the invalid parameters, and false when a later call may do better.
func (s *Source) build(ctx context.Context, in input) (bool, []string, *config.Settings) {
	cfg, err := config.Parse([]byte(in.config))
	if err != nil {
		s.logger.Error("agent configuration is not valid, the previous one stays", "error", err)

		return true, []string{config.ParamConfig}, nil
	}

	prev := config.Defaults()
	if s.applied {
		prev = s.current.Config
	}

	cfg, invalid := cfg.Resolve(prev)
	if len(invalid) > 0 {
		s.logger.Error("agent configuration has values out of range, the previous ones stay", "parameters", invalid)
	}

	if !cfg.Complete() {
		return true, invalid, nil
	}

	mode := cfg.ModeOf(in.group)
	if !config.ValidMode(mode) {
		s.logger.Error("mode of the node group is not valid, the previous configuration stays", "group", in.group, "mode", mode)

		return true, append(invalid, config.ParamMode), nil
	}

	st := config.Settings{Config: cfg, NodeGroup: in.group, Mode: mode, Engine: cfg.EngineOf(in.group), ProfileName: cfg.ProfileName(mode)}

	st.Profile, err = s.fetch(ctx, st.ProfileName, cfg)
	if err == nil {
		st.Memberlist = st.Profile
		if cfg.MemberlistProfile != st.ProfileName {
			st.Memberlist, err = s.fetch(ctx, cfg.MemberlistProfile, cfg)
		}
	}

	switch {
	case errors.Is(err, profile.ErrInvalid):
		s.logger.Error("profile is not valid, the previous one stays", "error", err)

		return true, append(invalid, config.ParamProfile), nil
	case err != nil:
		s.logger.Warn("profile is not ready, the previous one stays", "error", err)

		return false, invalid, nil
	}

	return true, invalid, &st
}

func (s *Source) fetch(ctx context.Context, name string, cfg config.Config) (v1alpha1.FencingSLAProfileSpec, error) {
	return profile.Fetch(ctx, s.profiles, name, cfg.ProfileChecksums[name])
}

// report sets the config_invalid metric of the invalid parameters and clears the rest.
func (s *Source) report(invalid []string) {
	for _, p := range config.Params {
		s.metrics.ConfigInvalid(p, slices.Contains(invalid, p))
	}
}
