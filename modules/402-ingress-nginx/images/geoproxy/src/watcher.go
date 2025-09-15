/*
Copyright 2025 Flant JSC

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

package geodownloader

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"time"
)

// LicenseEditions { "<license>": ["GeoLite2-City","GeoLite2-ASN", ...], ... }
type LicenseEditions map[string][]string

type GeoUpdaterSecret struct {
	Ready           bool
	secret          *v1.Secret
	licenseEditions LicenseEditions
	handler         *Handler
	Updated         chan struct{}
}

func NewGeoUpdaterSecret(handler *Handler) *GeoUpdaterSecret {
	g := &GeoUpdaterSecret{
		handler: handler,
		Updated: make(chan struct{}, 1),
	}
	return g
}

func (g *GeoUpdaterSecret) RunWatcher(ctx context.Context, secretName, secretNamespace string) error {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		1*time.Minute, // Resync interval
		informers.WithNamespace(secretNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("metadata.name=%s", secretName)
		}),
	)

	secretInformer := factory.Core().V1().Secrets().Informer()

	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*v1.Secret)
			err := g.getLicenseEditionsFromSecret(secret)
			if err != nil {
				log.Error(fmt.Sprintf("Failed to get license editions for secret %s: %v", secretName, err))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newSecret := newObj.(*v1.Secret)
			oldSecret := oldObj.(*v1.Secret)
			if newSecret.ResourceVersion == oldSecret.ResourceVersion {
				return
			}
			err := g.getLicenseEditionsFromSecret(newSecret)
			if err != nil {
				log.Error(fmt.Sprintf("Failed to get license editions for secret %s: %v", newSecret.Name, err))
			}
			select {
			case g.Updated <- struct{}{}: // send signal if secret was changed
			default:
			}
		},
		DeleteFunc: func(_ interface{}) {
			log.Warn(fmt.Sprintf("Secret: %s was be deleted", secretName))
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	<-ctx.Done()
	log.Info(fmt.Sprintf("Stopping watcher for secret %s", secretName))
	return nil
}

func (g *GeoUpdaterSecret) getLicenseEditionsFromSecret(secret *v1.Secret) error {
	if secret.Data == nil {
		return fmt.Errorf("secret data is nil")
	}

	var licenseEditions LicenseEditions

	if data, exists := secret.Data["license_editions.json"]; exists {
		if err := json.Unmarshal(data, &licenseEditions); err != nil {
			return err
		}
	}

	g.handler.Mu.Lock()
	g.licenseEditions = licenseEditions
	g.Ready = true
	g.handler.Cond.Broadcast()
	g.handler.Mu.Unlock()
	return nil
}

func (g *GeoUpdaterSecret) GetLicenseEditions() LicenseEditions {

	if g.licenseEditions == nil {
		g.licenseEditions = make(LicenseEditions)
	}

	return g.licenseEditions
}
