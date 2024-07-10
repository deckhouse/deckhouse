/*
Copyright 2024 Flant JSC

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

package kubernetesversion

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/square/go-jose/v3/json"
	"net/http"
	"os"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"github.com/flant/addon-operator/pkg/utils/logger"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
)

const (
	Name              extenders.ExtenderName = "KubernetesVersion"
	RequirementsField string                 = "kubernetes"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	logger         logger.Logger
	versionMatcher *versionmatcher.Matcher
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{logger: log.WithField("extender", Name), versionMatcher: versionmatcher.New()}
		instance.fetchKubernetesVersion()
	})
	return instance
}
func (e *Extender) fetchKubernetesVersion() {
	debugAddress := os.Getenv("DEBUG_HTTP_SERVER_ADDR")
	if debugAddress == "" {
		e.logger.Errorf("env DEBUG_HTTP_SERVER_ADDDR is not set")
		return
	}
	// TODO(ipaqsa): add retry
	resp, err := http.Get(fmt.Sprintf("http://%s/requirements", debugAddress))
	if err != nil || resp.StatusCode != http.StatusOK {
		e.logger.Errorf("error fetching kubernetes version: %v", err)
		return
	}
	values := map[string]interface{}{}
	if err = json.NewDecoder(resp.Body).Decode(&values); err != nil {
		e.logger.Errorf("error parsing requirements: %v", err)
		return
	}
	if len(values["global.discovery.kubernetesVersion"].(string)) > 0 {
		kubeVersion, err := semver.NewVersion(values["global.discovery.kubernetesVersion"].(string))
		if err != nil {
			e.logger.Errorf("error parsing kubernetes version: %v", err)
			return
		}
		e.versionMatcher.SetBaseVersion(kubeVersion)
		return
	}
	e.logger.Errorf("kubernetes version not found in requirements")
}

func (e *Extender) AddConstraint(name, rawConstraint string) error {
	if err := e.versionMatcher.AddConstraint(name, rawConstraint); err != nil {
		e.logger.Debugf("adding constraint for %q failed", name)
		return err
	}
	e.logger.Debugf("constraint for %q is added", name)
	return nil
}

func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	if err := e.versionMatcher.Validate(name); err != nil {
		e.logger.Errorf("requirements of %s are not satisfied: current kubernetes version is not suitable: %s", name, err.Error())
		return pointer.Bool(false), fmt.Errorf("requirements are not satisfied: current kubernetes version is not suitable: %s", err.Error())
	}
	e.logger.Debugf("requirements of %s are satisfied", name)
	return pointer.Bool(true), nil
}

func (e *Extender) IsTerminator() bool {
	return true
}
