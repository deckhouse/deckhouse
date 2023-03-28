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

package destination

import (
	"encoding/base64"

	"github.com/iancoleman/strcase"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
)

var _ apis.LogDestination = (*CommonSettings)(nil)

const (
	bufferMaxBytesMinimumValue uint32 = 268435488
)

type CommonSettings struct {
	Name        string      `json:"-"`
	Type        string      `json:"type"`
	Inputs      set.Set     `json:"inputs,omitempty"`
	Healthcheck Healthcheck `json:"healthcheck"`
	Buffer      *Buffer     `json:"buffer,omitempty"`
}

func (cs *CommonSettings) SetInputs(inp []string) {
	cs.Inputs.Add(inp...)
}

func (cs *CommonSettings) GetName() string {
	return cs.Name
}

type Healthcheck struct {
	Enabled bool `json:"enabled"`
}
type Encoding struct {
	ExceptFields    []string `json:"except_fields,omitempty"`
	OnlyFields      []string `json:"only_fields,omitempty"`
	Codec           string   `json:"codec,omitempty"`
	TimestampFormat string   `json:"timestamp_format,omitempty"`
}

type CommonTLS struct {
	CAFile            string `json:"ca_file,omitempty"`
	CertFile          string `json:"crt_file,omitempty"`
	KeyFile           string `json:"key_file,omitempty"`
	KeyPass           string `json:"key_pass,omitempty"`
	VerifyHostname    bool   `json:"verify_hostname"`
	VerifyCertificate bool   `json:"verify_certificate"`

	Enabled bool `json:"enabled,omitempty"`
}

type Buffer struct {
	MaxSize   uint32 `json:"max_size,omitempty"`
	Type      string `json:"type,omitempty"`
	MaxEvents uint32 `json:"max_events,omitempty"`
	WhenFull  string `json:"when_full,omitempty"`
}

func decodeB64(input string) string {
	res, _ := base64.StdEncoding.DecodeString(input)
	return string(res)
}

func ComposeName(n string) string {
	return "destination/cluster/" + n
}

// buildVectorBuffer generates buffer config for vector if CRD buffer config is set
func buildVectorBuffer(buffer *v1alpha1.Buffer) *Buffer {
	if buffer != nil {
		return buildVectorBufferNotNil(buffer)
	}
	return nil
}

// buildVectorBufferNotNil generates buffer config for vector
// There is no need to validation, because there is already validation on CRD site
func buildVectorBufferNotNil(buffer *v1alpha1.Buffer) *Buffer {
	switch buffer.Type {
	case v1alpha1.BufferTypeDisk:
		maxBytes := uint32(buffer.Disk.MaxSize.Value())
		if maxBytes < bufferMaxBytesMinimumValue {
			maxBytes = bufferMaxBytesMinimumValue
		}
		return &Buffer{
			Type:     toVectorValue(v1alpha1.BufferTypeDisk),
			MaxSize:  maxBytes,
			WhenFull: toVectorValue(buffer.WhenFull),
		}
	case v1alpha1.BufferTypeMemory:
		return &Buffer{
			Type:      toVectorValue(v1alpha1.BufferTypeMemory),
			MaxEvents: buffer.Memory.MaxEvents,
			WhenFull:  toVectorValue(buffer.WhenFull),
		}
	}
	return nil
}

// toVectorValue converts string to snake case
// it is a contract between Deckhouse and vector: Deckhouse uses upper kebap, vector uses snake case.
func toVectorValue(t string) string {
	return strcase.ToSnake(t)
}
