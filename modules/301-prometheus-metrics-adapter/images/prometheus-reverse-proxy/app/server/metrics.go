/*
Copyright 2022 Flant JSC

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

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const configPath = "/etc/prometheus-reverse-proxy/reverse-proxy.json"

var (
	appliedConfigMtime int64 = 0

	mu     sync.RWMutex
	config map[string]map[string]CustomMetricConfig
)

// Find namespaced patterns in request URIs
var (
	reNamespaceMatcher      = regexp.MustCompile(`.*namespace="([0-9a-zA-Z_\-]+)".*`)
	reMultiNamespaceMatcher = regexp.MustCompile(`.*namespace=~.*`)
)

type CustomMetricConfig struct {
	Cluster    string            `json:"cluster"`
	Namespaced map[string]string `json:"namespaced"`
}

type MetricHandler struct {
	ObjectType    string
	MetricName    string
	Selector      string
	GroupBy       string
	Namespace     string
	QueryTemplate string

	MetricConfig CustomMetricConfig
}

func (m *MetricHandler) RenderQuery() string {
	// TODO(nabokihms): use go template
	query := strings.Replace(m.QueryTemplate, "<<.LabelMatchers>>", m.Selector, -1)
	query = strings.Replace(query, "<<.GroupBy>>", m.GroupBy, -1)

	return query
}

func (m *MetricHandler) Init() error {
	namespaceMatch := reNamespaceMatcher.FindStringSubmatch(m.Selector)

	if namespaceMatch != nil {
		m.Namespace = namespaceMatch[1]
	} else {
		if reMultiNamespaceMatcher.MatchString(m.Selector) {
			return fmt.Errorf("multiple namespaces are not implemented, selector: %s", m.Selector)
		} else {
			return fmt.Errorf("no 'namespace=' label in selector '%s' given", m.Selector)
		}
	}

	mu.RLock()
	defer mu.RUnlock()

	if metricConfig, ok := config[m.ObjectType][m.MetricName]; ok {
		m.MetricConfig = metricConfig
	} else {
		return fmt.Errorf("metric '%s' for object '%s' not configured", m.MetricName, m.ObjectType)
	}

	if queryTemplate, ok := m.MetricConfig.Namespaced[m.Namespace]; ok {
		m.QueryTemplate = queryTemplate
	} else if len(m.MetricConfig.Cluster) > 0 {
		m.QueryTemplate = m.MetricConfig.Cluster
	} else {
		return fmt.Errorf("metric '%s' for object '%s' not configured for namespace '%s' or cluster-wide",
			m.MetricName, m.ObjectType, m.Namespace)
	}

	return nil
}

func updateConfig() {
	fStat, _ := os.Stat(configPath)
	if mtime := fStat.ModTime().Unix(); mtime != appliedConfigMtime {
		f, _ := os.Open(configPath)
		defer f.Close()

		mu.Lock()
		defer mu.Unlock()

		json.NewDecoder(f).Decode(&config)
		appliedConfigMtime = mtime
	}
}

func StartConfigUpdater() {
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		errLog.Fatalf("config file %s does not exist", configPath)
	}

	updateConfig()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		for _ = range ticker.C {
			updateConfig()
		}
	}()
}
