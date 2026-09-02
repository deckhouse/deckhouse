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

package template

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	cloudProviderNameLabel            = "cloud-provider.deckhouse.io/name"
	cloudProviderBashibleLabel        = "cloud-provider.deckhouse.io/bashible"
	cloudProviderStepsLabelSelector   = cloudProviderBashibleLabel + "=steps," + cloudProviderNameLabel
	cloudProviderStepsSecretNamespace = "kube-system"
)

type cloudProviderStepsSecret struct {
	provider string
	data     map[string][]byte
}

func (s *StepsStorage) subscribeOnCloudProviderSteps(
	ctx context.Context,
	factory informers.SharedInformerFactory,
) {
	if factory == nil {
		return
	}

	informer := factory.Core().V1().Secrets().Informer()
	_ = informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	_, _ = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return
			}
			s.upsertCloudProviderStepsSecret(secret)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSecret, oldOK := oldObj.(*corev1.Secret)
			newSecret, newOK := newObj.(*corev1.Secret)
			if !oldOK || !newOK {
				return
			}
			if oldSecret.ResourceVersion == newSecret.ResourceVersion {
				return
			}
			s.upsertCloudProviderStepsSecret(newSecret)
		},
		DeleteFunc: func(obj interface{}) {
			secret, ok := deletedSecret(obj)
			if !ok {
				return
			}
			s.deleteCloudProviderStepsSecret(secret.Name)
		},
	})

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		klog.Errorf("unable to sync cloud-provider steps Secrets informer: %v", ctx.Err())
	}
}

func (s *StepsStorage) upsertCloudProviderStepsSecret(secret *corev1.Secret) {
	provider := secret.Labels[cloudProviderNameLabel]
	if provider == "" {
		klog.Warningf("Ignoring cloud-provider steps Secret %q without %q label", secret.Name, cloudProviderNameLabel)
		return
	}

	scripts := make(map[string][]byte)
	for name, content := range secret.Data {
		if !strings.HasSuffix(name, ".sh.tpl") {
			continue
		}
		scripts[name] = append([]byte(nil), content...)
	}

	s.m.Lock()
	s.cloudProviderStepSecrets[secret.Name] = cloudProviderStepsSecret{
		provider: provider,
		data:     scripts,
	}
	s.m.Unlock()

	s.notifyCloudProviderStepsChanged()
}

func (s *StepsStorage) deleteCloudProviderStepsSecret(name string) {
	s.m.Lock()
	_, exists := s.cloudProviderStepSecrets[name]
	if exists {
		delete(s.cloudProviderStepSecrets, name)
	}
	s.m.Unlock()

	if exists {
		s.notifyCloudProviderStepsChanged()
	}
}

func (s *StepsStorage) notifyCloudProviderStepsChanged() {
	select {
	case s.cloudProviderStepsChanged <- struct{}{}:
	default:
	}
}

func (s *StepsStorage) OnCloudProviderStepsChanged() <-chan struct{} {
	return s.cloudProviderStepsChanged
}

func deletedSecret(obj interface{}) (*corev1.Secret, bool) {
	if secret, ok := obj.(*corev1.Secret); ok {
		return secret, true
	}

	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}

	secret, ok := tombstone.Obj.(*corev1.Secret)
	return secret, ok
}

func (s *StepsStorage) cloudProviderStepsFor(provider string) (map[string][]byte, bool, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	scripts := make(map[string][]byte)
	owners := make(map[string]string)
	found := false

	for secretName, secret := range s.cloudProviderStepSecrets {
		if secret.provider != provider {
			continue
		}

		found = true

		for scriptName, content := range secret.data {
			if owner, exists := owners[scriptName]; exists {
				return nil, true, fmt.Errorf(
					"cloud-provider steps Secrets %q and %q contain the same script %q",
					owner,
					secretName,
					scriptName,
				)
			}

			owners[scriptName] = secretName
			scripts[scriptName] = append([]byte(nil), content...)
		}
	}

	return scripts, found, nil
}
