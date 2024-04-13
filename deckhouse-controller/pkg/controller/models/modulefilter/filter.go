/*
Copyright 2023 Flant JSC

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

package modulefilter

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned"
	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/client/clientset/versioned/typed/deckhouse.io/v1alpha1"
)

func New(mm *module_manager.ModuleManager) *Filter {
	return &Filter{mm: mm, externalModulesDir: os.Getenv("EXTERNAL_MODULES_DIR")}
}

type Filter struct {
	mm                 *module_manager.ModuleManager
	externalModulesDir string
}

func (f *Filter) IsEmbeddedModule(moduleName string) bool {
	m := f.mm.GetModule(moduleName)
	if m == nil {
		log.Warnf("module %s not found", moduleName)
		return false
	}

	log.Infof("TMP: %v %v %v", moduleName, m.Path, !strings.Contains(m.Path, f.externalModulesDir))
	return !strings.Contains(m.Path, f.externalModulesDir)
}

func NewAPI(config *rest.Config) (*APIFilter, error) {
	mcClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("new versioned client: %w", err)
	}

	return &APIFilter{client: mcClient.DeckhouseV1alpha1().Modules()}, nil
}

type APIFilter struct {
	client deckhousev1alpha1.ModuleInterface
}

func (f *APIFilter) IsEmbeddedModule(moduleName string) bool {
	module, err := f.client.Get(context.Background(), moduleName, metav1.GetOptions{})
	if err != nil {
		log.Error("get module %s: %s", moduleName, err)
		return false
	}

	log.Infof("TMP-2: %v %v", moduleName, module.Properties.Source == "Embedded")
	return module.Properties.Source == "Embedded"
}
