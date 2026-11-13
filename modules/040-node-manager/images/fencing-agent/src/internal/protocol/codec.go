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

// Package protocol encodes the peer protocol messages and the NodeMeta, keeps
// the memberlist keyring and sends and receives the messages through memberlist.
package protocol

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"google.golang.org/protobuf/encoding/protowire"

	"fencing-agent/internal/domain"
)

// The wire format is the protobuf encoding. Field numbers never change and are
// never used again. A new field gets a new number.
//
// Envelope: 1 type, 2 version, 3 from, 4 body (the fields of the type).

const (
	fieldType    protowire.Number = 1
	fieldVersion protowire.Number = 2
	fieldFrom    protowire.Number = 3
	fieldBody    protowire.Number = 4
)

type msgType uint64

const (
	typeQuestion msgType = iota + 1
	typeAnswer
	typeDeclareSuicide
	typeSeeCP
	typeLeaving
	typeAsked
	typeFFNSCreated
)

// errUnknownType is the error for a message of a type that this version does not know.
var errUnknownType = errors.New("unknown message type")

// Encode encodes msg with the header h. Encode does not use the header in msg.
func Encode(h domain.Header, msg domain.Message) ([]byte, error) {
	var (
		typ  msgType
		body []byte
	)

	switch m := msg.(type) {
	case domain.Question:
		typ = typeQuestion
		body = appendFixed64(body, 1, m.PollID)
		body = appendString(body, 2, string(m.Network))
		body = appendVarint(body, 3, m.Epoch)
		body = appendString(body, 4, m.To)
		if m.Addr.IsValid() {
			body = appendBytes(body, 5, m.Addr.AsSlice())
		}
		body = appendDuration(body, 6, m.LastReadyAgo)
	case domain.Answer:
		typ = typeAnswer
		body = appendString(body, 1, m.To)
		body = appendFixed64(body, 2, m.PollID)
		body = appendVarint(body, 3, m.Epoch)
		body = appendString(body, 4, string(m.Network))
		body = appendVarint(body, 5, uint64(m.Verdict))
		body = appendDuration(body, 6, m.LastReadyAgo)
	case domain.DeclareSuicide:
		typ = typeDeclareSuicide
		body = appendFixed64(body, 1, m.PollID)
	case domain.SeeCP:
		typ = typeSeeCP
	case domain.Leaving:
		typ = typeLeaving
	case domain.Asked:
		typ = typeAsked
		body = appendString(body, 1, m.Node)
		if !m.At.IsZero() {
			body = appendVarint(body, 2, protowire.EncodeZigZag(m.At.UnixNano()))
		}
	case domain.FFNSCreated:
		typ = typeFFNSCreated
		body = appendString(body, 1, m.Node)
	default:
		return nil, fmt.Errorf("encode: unsupported message %T", msg)
	}

	b := appendVarint(nil, fieldType, uint64(typ))
	b = appendVarint(b, fieldVersion, uint64(h.Version))
	b = appendString(b, fieldFrom, h.From)
	b = appendBytes(b, fieldBody, body)

	return b, nil
}

// Decode decodes a message. A field that this version does not know is
// skipped; in an Answer it sets UnknownFields. A message of an unknown type
// gives errUnknownType.
func Decode(b []byte) (domain.Message, error) {
	envelope, err := parse(b)
	if err != nil {
		return nil, err
	}

	var (
		typ     msgType
		h       domain.Header
		body    []byte
		unknown bool
	)

	for _, f := range envelope {
		switch {
		case f.is(fieldType, protowire.VarintType):
			typ = msgType(f.v)
		case f.is(fieldVersion, protowire.VarintType):
			h.Version = uint32(f.v)
		case f.is(fieldFrom, protowire.BytesType):
			h.From = string(f.b)
		case f.is(fieldBody, protowire.BytesType):
			body = f.b
		default:
			unknown = true
		}
	}

	fields, err := parse(body)
	if err != nil {
		return nil, err
	}

	switch typ {
	case typeQuestion:
		q := domain.Question{Header: h}
		for _, f := range fields {
			switch {
			case f.is(1, protowire.Fixed64Type):
				q.PollID = f.v
			case f.is(2, protowire.BytesType):
				q.Network = domain.Network(f.b)
			case f.is(3, protowire.VarintType):
				q.Epoch = f.v
			case f.is(4, protowire.BytesType):
				q.To = string(f.b)
			case f.is(5, protowire.BytesType):
				q.Addr, _ = netip.AddrFromSlice(f.b)
			case f.is(6, protowire.VarintType):
				q.LastReadyAgo = duration(f.v)
			}
		}

		return q, nil
	case typeAnswer:
		a := domain.Answer{Header: h}
		for _, f := range fields {
			switch {
			case f.is(1, protowire.BytesType):
				a.To = string(f.b)
			case f.is(2, protowire.Fixed64Type):
				a.PollID = f.v
			case f.is(3, protowire.VarintType):
				a.Epoch = f.v
			case f.is(4, protowire.BytesType):
				a.Network = domain.Network(f.b)
			case f.is(5, protowire.VarintType):
				a.Verdict = verdict(f.v)
			case f.is(6, protowire.VarintType):
				a.LastReadyAgo = duration(f.v)
			default:
				unknown = true
			}
		}
		a.UnknownFields = unknown

		return a, nil
	case typeDeclareSuicide:
		m := domain.DeclareSuicide{Header: h}
		for _, f := range fields {
			if f.is(1, protowire.Fixed64Type) {
				m.PollID = f.v
			}
		}

		return m, nil
	case typeSeeCP:
		return domain.SeeCP{Header: h}, nil
	case typeLeaving:
		return domain.Leaving{Header: h}, nil
	case typeAsked:
		m := domain.Asked{Header: h}
		for _, f := range fields {
			switch {
			case f.is(1, protowire.BytesType):
				m.Node = string(f.b)
			case f.is(2, protowire.VarintType):
				m.At = time.Unix(0, protowire.DecodeZigZag(f.v))
			}
		}

		return m, nil
	case typeFFNSCreated:
		m := domain.FFNSCreated{Header: h}
		for _, f := range fields {
			if f.is(1, protowire.BytesType) {
				m.Node = string(f.b)
			}
		}

		return m, nil
	default:
		return nil, errUnknownType
	}
}

// NodeMeta fields: 1 bootID, 2 armed, 3 nodeUID, 5 keyID, 6 leaving,
// 7 maintenance, 8 protocolVersion, 10 nodeGroup, 11 profile.
// TODO: the configuration confirmation gets the number 9.

// EncodeMeta encodes the NodeMeta.
func EncodeMeta(m domain.NodeMeta) []byte {
	var b []byte
	b = appendString(b, 1, m.BootID)
	b = appendBool(b, 2, m.Armed)
	b = appendString(b, 3, m.NodeUID)
	b = appendString(b, 5, m.KeyID)
	b = appendBool(b, 6, m.Leaving)
	b = appendBool(b, 7, m.Maintenance)
	b = appendVarint(b, 8, uint64(m.ProtocolVersion))
	b = appendString(b, 10, m.NodeGroup)
	b = appendString(b, 11, m.Profile)

	return b
}

// DecodeMeta decodes the NodeMeta. It skips the fields that it does not know.
func DecodeMeta(b []byte) (domain.NodeMeta, error) {
	fields, err := parse(b)
	if err != nil {
		return domain.NodeMeta{}, err
	}

	var m domain.NodeMeta
	for _, f := range fields {
		switch {
		case f.is(1, protowire.BytesType):
			m.BootID = string(f.b)
		case f.is(2, protowire.VarintType):
			m.Armed = protowire.DecodeBool(f.v)
		case f.is(3, protowire.BytesType):
			m.NodeUID = string(f.b)
		case f.is(5, protowire.BytesType):
			m.KeyID = string(f.b)
		case f.is(6, protowire.VarintType):
			m.Leaving = protowire.DecodeBool(f.v)
		case f.is(7, protowire.VarintType):
			m.Maintenance = protowire.DecodeBool(f.v)
		case f.is(8, protowire.VarintType):
			m.ProtocolVersion = uint32(f.v)
		case f.is(10, protowire.BytesType):
			m.NodeGroup = string(f.b)
		case f.is(11, protowire.BytesType):
			m.Profile = string(f.b)
		}
	}

	return m, nil
}

// verdict maps the wire value to a Verdict. A value that this version does
// not know gives VerdictUnknown.
func verdict(v uint64) domain.Verdict {
	switch v {
	case uint64(domain.VerdictSee), uint64(domain.VerdictNotSee), uint64(domain.VerdictDying):
		return domain.Verdict(v)
	default:
		return domain.VerdictUnknown
	}
}

func duration(v uint64) time.Duration {
	return time.Duration(protowire.DecodeZigZag(v))
}

type field struct {
	num protowire.Number
	typ protowire.Type
	// v is the value of a varint or fixed field, b is the value of a bytes field.
	v uint64
	b []byte
}

func (f field) is(num protowire.Number, typ protowire.Type) bool {
	return f.num == num && f.typ == typ
}

// parse splits b into fields.
func parse(b []byte) ([]field, error) {
	var fields []field
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		f := field{num: num, typ: typ}
		switch typ {
		case protowire.VarintType:
			f.v, n = protowire.ConsumeVarint(b)
		case protowire.Fixed64Type:
			f.v, n = protowire.ConsumeFixed64(b)
		case protowire.BytesType:
			f.b, n = protowire.ConsumeBytes(b)
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
		}
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		fields = append(fields, f)
	}

	return fields, nil
}

// The append functions skip zero values, as protobuf does.

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)

	return protowire.AppendVarint(b, v)
}

func appendFixed64(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.Fixed64Type)

	return protowire.AppendFixed64(b, v)
}

func appendBytes(b []byte, num protowire.Number, v []byte) []byte {
	if len(v) == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)

	return protowire.AppendBytes(b, v)
}

func appendString(b []byte, num protowire.Number, v string) []byte {
	if v == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)

	return protowire.AppendString(b, v)
}

func appendBool(b []byte, num protowire.Number, v bool) []byte {
	return appendVarint(b, num, protowire.EncodeBool(v))
}

func appendDuration(b []byte, num protowire.Number, d time.Duration) []byte {
	return appendVarint(b, num, protowire.EncodeZigZag(int64(d)))
}
