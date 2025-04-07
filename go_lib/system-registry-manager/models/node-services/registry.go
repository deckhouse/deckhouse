/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

// Registry holds detailed configuration of the registry
type Registry struct {
	UserRW     User              `json:"user_rw,omitempty" yaml:"user_rw,omitempty"`
	UserRO     User              `json:"user_ro,omitempty" yaml:"user_ro,omitempty"`
	Upstream   *UpstreamRegistry `json:"upstream,omitempty" yaml:"upstream,omitempty"`
	HTTPSecret string            `json:"http_secret,omitempty" yaml:"http_secret,omitempty"`
	Mirrorer   *Mirrorer         `json:"mirrorer,omitempty" yaml:"mirrorer,omitempty"`
}

func (rd Registry) Validate() error {
	var fields []*validation.FieldRules

	fields = append(fields, validation.Field(&rd.HTTPSecret, validation.Required))
	fields = append(fields, validation.Field(&rd.UserRO, validation.Required))
	fields = append(fields, validation.Field(&rd.UserRW, validation.Required))

	fields = append(fields, validation.Field(&rd.Mirrorer))
	fields = append(fields, validation.Field(&rd.Upstream))

	return validation.ValidateStruct(&rd, fields...)
}
