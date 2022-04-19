/*
Copyright 2021 Flant JSC

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

package model

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/impl"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

var validMustacheTemplate = regexp.MustCompile(`^\{\{\ ([a-zA-Z0-9][a-zA-Z0-9\[\]_\\\-\.]+)\ \}\}$`)

type commonDestinationSettings struct {
	Name        string   `json:"-"`
	Type        string   `json:"type"`
	Inputs      []string `json:"inputs,omitempty"`
	Healthcheck struct {
		Enabled bool `json:"enabled"`
	} `json:"healthcheck"`
	Buffer buffer `json:"buffer,omitempty"`
}

type buffer struct {
	Size uint32 `json:"max_size,omitempty"`
	Type string `json:"type,omitempty"`
}

type region struct {
	Region string `json:"region,omitempty"`
}

// AppendInputs append inputs to destination. If input is already exists - skip it (dedup)
func (cs *commonDestinationSettings) AppendInputs(inp []string) {
	if len(cs.Inputs) == 0 {
		cs.Inputs = inp
		return
	}

	m := make(map[string]bool, len(cs.Inputs))
	for _, d := range cs.Inputs {
		m[d] = true
	}

	for _, newinp := range inp {
		if _, ok := m[newinp]; !ok {
			cs.Inputs = append(cs.Inputs, newinp)
		}
	}
}

func (cs *commonDestinationSettings) GetName() string {
	return cs.Name
}

func NewLokiDestination(name string, cspec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	spec := cspec.Loki
	common := commonDestinationSettings{
		Name: "d8_cluster_sink_" + name,
		Type: "loki",
	}

	// Disable buffer. It is buggy. Vector developers know about problems with buffer.
	// More info about buffer rewriting here - https://github.com/vectordotdev/vector/issues/9476
	// common.Buffer = buffer{
	//	Size: 100 * 1024 * 1024, // 100MiB in bytes for vector persistent queue
	//	Type: "disk",
	// }

	LokiENC := LokiEncoding{
		Codec:           "text",
		TimestampFormat: "rfc3339",
		OnlyFields:      []string{"message"},
	}

	if spec.Auth.Password != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.Auth.Password)
		spec.Auth.Password = string(res)
	}

	if spec.TLS.CAFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CAFile)
		spec.TLS.CAFile = string(res)
	}

	if spec.TLS.CertFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CertFile)
		spec.TLS.CertFile = string(res)
	}

	if spec.TLS.KeyFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyFile)
		spec.TLS.KeyFile = string(res)
	}

	if spec.TLS.KeyPass != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyPass)
		spec.TLS.KeyPass = string(res)
	}

	if spec.Auth.Strategy != "" {
		spec.Auth.Strategy = strings.ToLower(spec.Auth.Strategy)
	}

	// default labels
	//
	// Asterisk is required here to expand all pod labels
	// See https://github.com/vectordotdev/vector/pull/12041
	labels := map[string]string{
		"namespace":    "{{ namespace }}",
		"container":    "{{ container }}",
		"image":        "{{ image }}",
		"pod":          "{{ pod }}",
		"node":         "{{ node }}",
		"pod_ip":       "{{ pod_ip }}",
		"stream":       "{{ stream }}",
		"pod_labels_*": "{{ pod_labels }}",
		"pod_owner":    "{{ pod_owner }}",
	}
	var dataField string
	keys := make([]string, 0, len(cspec.ExtraLabels))
	for key := range cspec.ExtraLabels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if validMustacheTemplate.MatchString(cspec.ExtraLabels[k]) {
			dataField = validMustacheTemplate.FindStringSubmatch(cspec.ExtraLabels[k])[1]
			labels[k] = fmt.Sprintf("{{ parsed_data.%s }}", dataField)
		} else {
			labels[k] = cspec.ExtraLabels[k]
		}
	}

	ctls := CommonTLS{
		CAFile:         spec.TLS.CAFile,
		CertFile:       spec.TLS.CertFile,
		KeyFile:        spec.TLS.KeyFile,
		KeyPass:        spec.TLS.KeyPass,
		VerifyHostname: spec.TLS.VerifyHostname,
	}

	return &lokiDestination{
		commonDestinationSettings: common,
		Auth:                      spec.Auth,
		TLS:                       ctls,
		Labels:                    labels,
		Endpoint:                  spec.Endpoint,
		Encoding:                  LokiENC,
		RemoveLabelFields:         true,
		OutOfOrderAction:          "rewrite_timestamp",
	}
}

func NewElasticsearchDestination(name string, cspec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	spec := cspec.Elasticsearch
	var ESBatch batch
	var BulkAction string = "index"

	common := commonDestinationSettings{
		Name: "d8_cluster_sink_" + name,
		Type: "elasticsearch",
	}

	// Disable buffer. It is buggy. Vector developers know about problems with buffer.
	// More info about buffer rewriting here - https://github.com/vectordotdev/vector/issues/9476
	// common.Buffer = buffer{
	//	Size: 100 * 1024 * 1024, // 100MiB in bytes for vector persistent queue
	//	Type: "disk",
	// }

	ESBatch = batch{
		MaxSize:     10 * 1024 * 1024, // 10MiB in bytes for elasticsearch bulk api
		TimeoutSecs: 1,
	}

	if spec.Auth.Password != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.Auth.Password)
		spec.Auth.Password = string(res)
	}

	if spec.Auth.AwsAccessKey != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.Auth.AwsAccessKey)
		spec.Auth.AwsAccessKey = string(res)
	}

	if spec.Auth.AwsSecretKey != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.Auth.AwsSecretKey)
		spec.Auth.AwsSecretKey = string(res)
	}

	if spec.TLS.CAFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CAFile)
		spec.TLS.CAFile = string(res)
	}

	if spec.TLS.CertFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CertFile)
		spec.TLS.CertFile = string(res)
	}

	if spec.TLS.KeyFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyFile)
		spec.TLS.KeyFile = string(res)
	}

	if spec.TLS.KeyPass != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyPass)
		spec.TLS.KeyPass = string(res)
	}

	EsEnc := ElasticsearchEncoding{
		TimestampFormat: "rfc3339",
	}

	EsAuth := ElasticsearchAuth{
		AwsAccessKey:  spec.Auth.AwsAccessKey,
		AwsSecretKey:  spec.Auth.AwsSecretKey,
		AwsAssumeRole: spec.Auth.AwsAssumeRole,
		User:          spec.Auth.User,
		Strategy:      spec.Auth.Strategy,
		Password:      spec.Auth.Password,
	}

	if EsAuth.Strategy != "" {
		EsAuth.Strategy = strings.ToLower(EsAuth.Strategy)
	}

	AwsRegion := region{
		Region: spec.Auth.AwsRegion,
	}

	ctls := CommonTLS{
		CAFile:         spec.TLS.CAFile,
		CertFile:       spec.TLS.CertFile,
		KeyFile:        spec.TLS.KeyFile,
		KeyPass:        spec.TLS.KeyPass,
		VerifyHostname: spec.TLS.VerifyHostname,
	}

	if spec.DataStreamEnabled {
		BulkAction = "create"
	}

	return &elasticsearchDestination{
		commonDestinationSettings: common,
		Auth:                      EsAuth,
		Encoding:                  EsEnc,
		TLS:                       ctls,
		AWS:                       AwsRegion,
		Batch:                     ESBatch,
		Endpoint:                  spec.Endpoint,
		Compression:               "gzip",
		Index:                     spec.Index,
		Pipeline:                  spec.Pipeline,
		BulkAction:                BulkAction,
		DocType:                   spec.DocType,
		// We do not neet this field for vector 0.14
		//Mode:                      "normal",
	}
}

func NewLogstashDestination(name string, cspec v1alpha1.ClusterLogDestinationSpec) impl.LogDestination {
	spec := cspec.Logstash
	var enabledTLS bool

	common := commonDestinationSettings{
		Name: "d8_cluster_sink_" + name,
		Type: "socket",
	}

	// Disable buffer. It is buggy. Vector developers know about problems with buffer.
	// More info about buffer rewriting here - https://github.com/vectordotdev/vector/issues/9476
	// common.Buffer = buffer{
	//	Size: 100 * 1024 * 1024, // 100MiB in bytes for vector persistent queue
	//	Type: "disk",
	// }

	if spec.TLS.CAFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CAFile)
		spec.TLS.CAFile = string(res)
	}

	if spec.TLS.CertFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.CertFile)
		spec.TLS.CertFile = string(res)
	}

	if spec.TLS.KeyFile != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyFile)
		spec.TLS.KeyFile = string(res)
	}

	if spec.TLS.KeyPass != "" {
		res, _ := base64.StdEncoding.DecodeString(spec.TLS.KeyPass)
		spec.TLS.KeyPass = string(res)
	}

	if spec.TLS.KeyFile != "" || spec.TLS.CertFile != "" || spec.TLS.CAFile != "" {
		enabledTLS = true
	} else {
		enabledTLS = false
	}
	ctls := CommonTLS{
		CAFile:         spec.TLS.CAFile,
		CertFile:       spec.TLS.CertFile,
		KeyFile:        spec.TLS.KeyFile,
		KeyPass:        spec.TLS.KeyPass,
		VerifyHostname: spec.TLS.VerifyHostname,
	}
	lstls := LogstashTLS{
		CommonTLS:         ctls,
		VerifyCertificate: spec.TLS.VerifyCertificate,
		Enabled:           enabledTLS,
	}
	logstashEnc := LogstashEncoding{
		Codec:           "json",
		TimestampFormat: "rfc3339",
	}
	keepalive := LogstashKeepalive{
		TimeSecs: 7200,
	}

	return &logstashDestination{
		commonDestinationSettings: common,
		Encoding:                  logstashEnc,
		TLS:                       lstls,
		Mode:                      "tcp",
		Address:                   spec.Endpoint,
		Keepalive:                 keepalive,
	}
}
