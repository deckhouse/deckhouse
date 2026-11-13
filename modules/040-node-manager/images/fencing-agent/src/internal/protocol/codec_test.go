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

package protocol

import (
	"errors"
	"math"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"

	"fencing-agent/internal/domain"
)

var testHeader = domain.Header{Version: domain.ProtocolVersion, From: "node-a"}

func roundTrip(t *testing.T, msg domain.Message) domain.Message {
	t.Helper()

	b, err := Encode(testHeader, msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	return got
}

// The fields of each message survive the encoding: a question and an answer
// carry all fields, an answer has no relay flag.
func TestCodecRoundTrip(t *testing.T) {
	at := time.Date(2026, 10, 22, 10, 0, 0, 123, time.UTC)

	cases := []domain.Message{
		domain.Question{
			Header: testHeader, PollID: math.MaxUint64, Network: domain.NetworkMain, Epoch: 7,
			To: "node-b", Addr: netip.MustParseAddr("10.0.0.7"), LastReadyAgo: 1540 * time.Millisecond,
		},
		domain.Answer{
			Header: testHeader, To: "node-b", PollID: 42, Epoch: 7, Network: domain.NetworkSystem,
			Verdict: domain.VerdictSee, LastReadyAgo: 300 * time.Millisecond,
		},
		domain.Answer{Header: testHeader, To: "node-b", PollID: 1, Verdict: domain.VerdictNotSee},
		domain.Answer{Header: testHeader, To: "node-b", PollID: 1, Verdict: domain.VerdictDying},
		domain.DeclareSuicide{Header: testHeader, PollID: 42},
		domain.SeeCP{Header: testHeader},
		domain.Leaving{Header: testHeader},
		domain.Asked{Header: testHeader, Node: "node-c", At: at},
		domain.FFNSCreated{Header: testHeader, Node: "node-c"},
	}

	for _, want := range cases {
		got := roundTrip(t, want)
		if q, ok := got.(domain.Asked); ok {
			if !q.At.Equal(at) {
				t.Errorf("Asked.At = %v, want %v", q.At, at)
			}
			q.At = at
			got = q
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip of %T:\n got %+v\nwant %+v", want, got, want)
		}
	}
}

func TestEncodeUsesGivenHeader(t *testing.T) {
	got := roundTrip(t, domain.SeeCP{Header: domain.Header{Version: 99, From: "other"}})
	if got.(domain.SeeCP).Header != testHeader {
		t.Errorf("header = %+v, want %+v", got.(domain.SeeCP).Header, testHeader)
	}
}

// An answer with an unknown verdict.
func TestDecodeUnknownVerdict(t *testing.T) {
	for _, v := range []uint64{0, 4, 99, 257} {
		body := protowire.AppendTag(nil, 1, protowire.BytesType)
		body = protowire.AppendString(body, "node-b")
		if v != 0 {
			body = protowire.AppendTag(body, 5, protowire.VarintType)
			body = protowire.AppendVarint(body, v)
		}

		got, err := Decode(envelope(typeAnswer, body))
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if a := got.(domain.Answer); a.Verdict != domain.VerdictUnknown || a.UnknownFields {
			t.Errorf("verdict %d: got verdict %d, unknown fields %v; want VerdictUnknown without unknown fields", v, a.Verdict, a.UnknownFields)
		}
	}
}

func TestDecodeAnswerUnknownFields(t *testing.T) {
	known, err := Encode(testHeader, domain.Answer{To: "node-b", PollID: 1, Verdict: domain.VerdictSee})
	if err != nil {
		t.Fatal(err)
	}

	answerBody := func(extra []byte) []byte {
		body := protowire.AppendTag(nil, 1, protowire.BytesType)
		body = protowire.AppendString(body, "node-b")
		body = protowire.AppendTag(body, 5, protowire.VarintType)
		body = protowire.AppendVarint(body, uint64(domain.VerdictSee))

		return append(body, extra...)
	}

	cases := map[string][]byte{
		"new field in the answer": envelope(typeAnswer, answerBody(
			protowire.AppendVarint(protowire.AppendTag(nil, 20, protowire.VarintType), 1))),
		"known field with a new wire type": envelope(typeAnswer, answerBody(
			protowire.AppendFixed32(protowire.AppendTag(nil, 6, protowire.Fixed32Type), 1))),
		"new field in the envelope": protowire.AppendString(
			protowire.AppendTag(append([]byte(nil), known...), 9, protowire.BytesType), "x"),
	}

	for name, b := range cases {
		got, err := Decode(b)
		if err != nil {
			t.Fatalf("%s: Decode: %v", name, err)
		}
		a := got.(domain.Answer)
		if !a.UnknownFields {
			t.Errorf("%s: UnknownFields = false, want true", name)
		}
		if a.Verdict != domain.VerdictSee || a.To != "node-b" {
			t.Errorf("%s: known fields are lost: %+v", name, a)
		}
	}

	got, err := Decode(known)
	if err != nil {
		t.Fatal(err)
	}
	if got.(domain.Answer).UnknownFields {
		t.Error("UnknownFields = true for an answer of this version")
	}
}

func TestDecodeOtherTypesSkipUnknownFields(t *testing.T) {
	body := protowire.AppendTag(nil, 1, protowire.BytesType)
	body = protowire.AppendString(body, "node-c")
	body = protowire.AppendTag(body, 30, protowire.BytesType)
	body = protowire.AppendString(body, "new")

	got, err := Decode(envelope(typeFFNSCreated, body))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.(domain.FFNSCreated).Node != "node-c" {
		t.Errorf("got %+v", got)
	}
}

// A message of an unknown type.
func TestDecodeUnknownType(t *testing.T) {
	for _, typ := range []msgType{0, 8, 1000} {
		if _, err := Decode(envelope(typ, nil)); !errors.Is(err, errUnknownType) {
			t.Errorf("type %d: err = %v, want errUnknownType", typ, err)
		}
	}
}

func TestDecodeBrokenMessage(t *testing.T) {
	b, err := Encode(testHeader, domain.Question{PollID: 1, To: "node-b"})
	if err != nil {
		t.Fatal(err)
	}

	for _, broken := range [][]byte{b[:len(b)-1], {0xff}} {
		_, err := Decode(broken)
		if err == nil || errors.Is(err, errUnknownType) {
			t.Errorf("Decode(%x) err = %v, want a parse error", broken, err)
		}
	}
}

func TestMetaRoundTrip(t *testing.T) {
	want := domain.NodeMeta{
		BootID: "6f1c2c4e-2b0e-4d5c-9a56-0d1e7e3c5b11", Armed: true,
		NodeUID: "d1b4f0a2-1c3e-4f5a-8b7c-9d0e1f2a3b4c", KeyID: "0123456789abcdef",
		Leaving: true, Maintenance: true, ProtocolVersion: domain.ProtocolVersion,
		NodeGroup: "worker", Profile: "critical",
	}

	got, err := DecodeMeta(EncodeMeta(want))
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// The limit values of the fields fit in 512 bytes. The test adds an extra
// field 9 with a value larger than a SHA-256 in hex.
func TestMetaLimitValues(t *testing.T) {
	uuid := strings.Repeat("f", 36)
	m := domain.NodeMeta{
		BootID: uuid, Armed: true, NodeUID: uuid,
		KeyID: strings.Repeat("f", 16), Leaving: true, Maintenance: true, ProtocolVersion: math.MaxUint32,
		NodeGroup: strings.Repeat("f", 63), Profile: "critical",
	}

	b := EncodeMeta(m)
	b = protowire.AppendTag(b, 9, protowire.BytesType)
	b = protowire.AppendString(b, strings.Repeat("f", 128))

	if len(b) > domain.MetaMaxSize {
		t.Fatalf("NodeMeta with limit values has %d bytes, the limit is %d", len(b), domain.MetaMaxSize)
	}

	got, err := DecodeMeta(b)
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}
	if got != m {
		t.Errorf("got %+v, want %+v", got, m)
	}
}

func envelope(typ msgType, body []byte) []byte {
	b := appendVarint(nil, fieldType, uint64(typ))
	b = appendVarint(b, fieldVersion, uint64(domain.ProtocolVersion)+1)
	b = appendString(b, fieldFrom, "node-next")

	return appendBytes(b, fieldBody, body)
}
